package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/arisros/ytta/internal/agent"
	"github.com/arisros/ytta/internal/install"
	"github.com/arisros/ytta/internal/store"
)

func runInstall(args []string, add bool) error {
	name := "uninstall"
	if add {
		name = "install"
	}
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	chosen := map[string]*bool{}
	for _, t := range targets {
		chosen[t.name] = fs.Bool(t.name, false, "target "+t.title)
	}
	rec := fs.Bool("record", false, "install the recorder hooks")
	apply := fs.Bool("apply", false, "write the change (default: preview only)")
	wrap := fs.Bool("wrap-statusline", false, "keep your own statusLine and record usage through it (Claude Code)")
	path := fs.String("settings", "", "the agent's settings or hooks file (default: its usual place)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	var t *target
	for i := range targets {
		if *chosen[targets[i].name] {
			if t != nil {
				return errors.New("name one agent at a time")
			}
			t = &targets[i]
		}
	}
	if t == nil {
		return errors.New("name the agent: " + targetFlags())
	}
	if *path == "" {
		*path = t.path()
	}
	claude := t.name == "claude"

	before, err := os.ReadFile(*path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	var after []byte
	if t.plugin != nil {
		return installPlugin(t, *path, before, add, *rec, *apply)
	}
	if t.block == nil && install.HasLegacy(before) {
		fmt.Println("Note: removing the entries " + install.LegacyMarker + " left behind; their binary is gone.")
	}
	if t.block != nil {
		if after, err = installBlock(t, before, add, *rec); err != nil {
			return err
		}
	} else if add {
		bin, err := selfPath()
		if err != nil {
			return err
		}
		live, recorded, note := t.events()
		if note != "" {
			fmt.Println("Note:", note)
		}
		h, events := install.Hook{Command: guarded(bin, "hook"+t.hookArgs), Timeout: t.timeout}, live
		if *rec {
			fmt.Printf("Note: the recorder replaces the live hooks; run install --%s again to return to them.\n", t.name)
			// Recording never feeds state, so it may run async and out of order.
			h, events = install.Hook{Command: guarded(bin, "hook --record"), Async: t.async, Timeout: t.timeout}, recorded
		}
		after, err = install.Add(before, events, h)
		if err != nil {
			return err
		}
		if claude && !*rec {
			var owned bool
			if after, owned, err = install.SetStatusLine(after, guarded(bin, "statusline")); err != nil {
				return err
			}
			if !owned && *wrap {
				if after, owned, err = install.WrapStatusLine(after, func(b64 string) string { return wrapping(bin, b64) }); err != nil {
					return err
				}
			}
			if !owned && !strings.Contains(string(after), install.WrapFlag) {
				fmt.Println("Note: you have your own statusLine, so ytta will not show token usage or plan limits.")
				fmt.Println("      Add --wrap-statusline to keep yours and record them through it.")
			}
		}
	} else {
		if after, err = install.Remove(before); err != nil {
			return err
		}
		if claude {
			if after, err = install.RemoveStatusLine(after); err != nil {
				return err
			}
		}
	}

	if string(before) == string(after) {
		fmt.Println("No change needed:", *path)
		return nil
	}
	if !*apply {
		fmt.Printf("Would update %s\n\n", *path)
		printDiff(before, after)
		fmt.Println("\nPreview only. Repeat with --apply to save.")
		return nil
	}
	backup, err := writeWithBackup(*path, before, after)
	if err != nil {
		return err
	}
	fmt.Println("Updated", *path)
	if backup != "" {
		fmt.Println("Backup:", backup)
	}
	if add {
		fmt.Println(t.afterApply)
	}
	return nil
}

// target is an agent whose hooks ytta can install.
type target struct {
	name, title string
	path        func() string
	// events returns the live and the recorder events, and a note to print.
	events     func() (live, recorded []string, note string)
	hookArgs   string
	async      bool // whether the agent's hooks take "async"
	timeout    int  // in the agent's own unit
	afterApply string
	// doctorNote follows the event count in `ytta doctor`.
	doctorNote string
	// plugin, when set, makes the target a single file ytta owns whole
	// instead of hook entries merged into the agent's settings. record asks
	// for the recorder in place of the live hooks.
	plugin func(bin string, record bool) []byte
	// block, when set, makes the target a block of text ytta owns at the end
	// of the agent's TOML or YAML config. rest is the file without that
	// block, and args is what the binary is called with.
	block func(rest, bin, args string) (string, error)
	// binary finds the ytta path a plugin file calls, for `ytta doctor`.
	binary func(file string) string
}

// installBlock adds or removes a target's block and returns the new file.
func installBlock(t *target, before []byte, add, record bool) ([]byte, error) {
	if !add {
		return install.RemoveBlock(before), nil
	}
	bin, err := selfPath()
	if err != nil {
		return nil, err
	}
	args := "hook" + t.hookArgs
	if record {
		fmt.Printf("Note: the recorder replaces the live hooks; run install --%s again to return to them.\n", t.name)
		args = "hook --record"
	}
	body, err := t.block(string(install.RemoveBlock(before)), bin, args)
	if err != nil {
		return nil, err
	}
	return install.AddBlock(before, body), nil
}

// installPlugin writes or removes a plugin file. ytta owns the whole
// file, and never touches one it did not write.
func installPlugin(t *target, path string, before []byte, add, record, apply bool) error {
	if before != nil && !strings.Contains(string(before), install.Marker) {
		return fmt.Errorf("%s is not ytta's plugin; move it away first", path)
	}
	var after []byte
	if add {
		bin, err := selfPath()
		if err != nil {
			return err
		}
		after = t.plugin(bin, record)
	}
	if string(before) == string(after) {
		fmt.Println("No change needed:", path)
		return nil
	}
	if !apply {
		verb := "write"
		if !add {
			verb = "remove"
		}
		fmt.Printf("Would %s %s\n\nPreview only. Repeat with --apply to save.\n", verb, path)
		return nil
	}
	if !add {
		if err := os.Remove(path); err != nil {
			return err
		}
		fmt.Println("Removed", path)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if _, err := writeWithBackup(path, before, after); err != nil {
		return err
	}
	fmt.Println("Wrote", path)
	fmt.Println(t.afterApply)
	return nil
}

func geminiSettingsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".gemini", "settings.json")
}

func opencodePluginPath() string { return configFile("opencode", "plugins", "ytta.js") }

func kiloPluginPath() string { return configFile("kilo", "plugin", "ytta.js") }

// configFile is a path under XDG_CONFIG_HOME, ~/.config when that is unset.
func configFile(elem ...string) string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(append([]string{base}, elem...)...)
}

// pluginSource fills a plugin's __YTTA__ with the binary's path.
func pluginSource(src, bin string) []byte {
	return []byte(strings.ReplaceAll(src, "__YTTA__", strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(bin)))
}

// pluginBinary is the path pluginSource filled in.
func pluginBinary(file string) string { return strings.TrimSpace(between(file, `const YTTA = "`, `"`)) }

var geminiEvents = []string{
	"SessionStart", "SessionEnd", "BeforeAgent", "AfterAgent", "BeforeModel", "BeforeTool", "AfterTool", "Notification",
}

var targets = []target{
	{
		name: "claude", title: "Claude Code", path: defaultSettingsPath, async: true, timeout: 5,
		events:     func() ([]string, []string, string) { return liveEvents, recordEvents, "" },
		afterApply: "Running Claude sessions pick this up on their next settings reload; restart one if it does not.",
	},
	{
		name: "codex", title: "Codex CLI", path: codexHooksPath, async: true, timeout: 5, hookArgs: " --agent codex",
		events:     codexEvents,
		doctorNote: "they run only once trusted with /hooks in Codex",
		afterApply: "Codex runs a new hook only after you trust it: open Codex and run /hooks. A changed command needs trusting again.",
	},
	{
		// Gemini's timeout is in milliseconds.
		name: "gemini", title: "Gemini CLI", path: geminiSettingsPath, timeout: 5000, hookArgs: " --agent gemini",
		events: func() ([]string, []string, string) { return geminiEvents, geminiEvents, "" },
		afterApply: "Restart Gemini CLI to load the hooks. If no agent shows up, its environment redaction is hiding TMUX_PANE from hooks: " +
			"allow that variable in Gemini's settings.",
	},
	{
		name: "opencode", title: "opencode", path: opencodePluginPath,
		plugin:     func(bin string, _ bool) []byte { return pluginSource(agent.OpenCodePlugin, bin) },
		binary:     pluginBinary,
		afterApply: "Restart opencode to load the plugin. It reports sessions of an opencode started in a tmux pane, not ones reached with opencode attach.",
	},
	{
		name: "copilot", title: "Copilot CLI", path: copilotHooksPath, plugin: copilotHooks,
		binary: func(file string) string {
			var f struct {
				Hooks map[string][]struct {
					Bash string `json:"bash"`
				} `json:"hooks"`
			}
			if json.Unmarshal([]byte(file), &f) != nil {
				return ""
			}
			for _, entries := range f.Hooks {
				for _, e := range entries {
					return guardedBinary(e.Bash)
				}
			}
			return ""
		},
		afterApply: "Restart Copilot CLI to load the hooks.",
	},
	{
		name: "droid", title: "Droid", path: homeFile(".factory", "settings.json"), timeout: 5, hookArgs: " --agent droid",
		events:     func() ([]string, []string, string) { return agent.DroidEvents, agent.DroidEvents, "" },
		afterApply: "Restart Droid to load the hooks.",
	},
	{
		name: "qwen", title: "Qwen Code", path: homeFile(".qwen", "settings.json"), async: true, timeout: 5, hookArgs: " --agent qwen",
		events:     func() ([]string, []string, string) { return agent.QwenEvents, agent.QwenEvents, "" },
		afterApply: "Restart Qwen Code to load the hooks.",
	},
	{
		name: "kilo", title: "Kilo Code", path: kiloPluginPath,
		plugin:     func(bin string, _ bool) []byte { return pluginSource(agent.KiloPlugin(), bin) },
		binary:     pluginBinary,
		afterApply: "Restart Kilo Code to load the plugin.",
	},
	{
		name: "pi", title: "Pi", path: homeFile(".pi", "agent", "extensions", "ytta.js"),
		plugin:     func(bin string, _ bool) []byte { return pluginSource(agent.PiExtension, bin) },
		binary:     pluginBinary,
		afterApply: "Restart Pi, or run /reload in it, to load the extension.",
	},
	{
		name: "kimi", title: "Kimi Code", path: homeFile(".kimi-code", "config.toml"), hookArgs: " --agent kimi",
		block:      kimiBlock,
		afterApply: "Restart Kimi Code to load the hooks.",
	},
	{
		name: "hermes", title: "Hermes Agent", path: homeFile(".hermes", "config.yaml"), hookArgs: " --agent hermes",
		block:      hermesBlock,
		afterApply: "Restart Hermes Agent. It asks once per hook before running it: accept each, or start it with --accept-hooks.",
	},
}

// An inline `hooks = [...]` cannot be followed by [[hooks]] tables.
var tomlInlineHooks = regexp.MustCompile(`(?m)^\s*hooks\s*=`)

// kimiBlock is one [[hooks]] table per event for Kimi Code's config.toml.
func kimiBlock(rest, bin, args string) (string, error) {
	if tomlInlineHooks.MatchString(rest) {
		return "", errors.New("the config defines hooks as an inline array, which [[hooks]] tables cannot extend; rewrite it as [[hooks]] tables first")
	}
	quoted := `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(guarded(bin, args)) + `"`
	var b strings.Builder
	for _, ev := range agent.KimiEvents {
		fmt.Fprintf(&b, "[[hooks]]\nevent = %q\ncommand = %s\ntimeout = 5\n\n", ev, quoted)
	}
	return b.String(), nil
}

var yamlHooksKey = regexp.MustCompile(`(?m)^hooks\s*:`)

// hermesBlock is the hooks key of Hermes Agent's config.yaml. Hermes runs a
// hook without a shell, so the command is the binary itself, unguarded: a
// missing one is a warning in Hermes and nothing more.
func hermesBlock(rest, bin, args string) (string, error) {
	if yamlHooksKey.MatchString(rest) {
		return "", errors.New("the config already has a hooks key, and YAML allows only one; add ytta's entries under it by hand, one per event: command " +
			shellQuote(bin) + " " + args)
	}
	quoted := `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(shellQuote(bin)+" "+args) + `"`
	var b strings.Builder
	b.WriteString("hooks:\n")
	for _, ev := range agent.HermesEvents {
		fmt.Fprintf(&b, "  %s:\n    - command: %s\n      timeout: 5\n", ev, quoted)
	}
	return b.String(), nil
}

// homeFile is a path under the home directory, resolved when it is asked for.
func homeFile(elem ...string) func() string {
	return func() string {
		home, _ := os.UserHomeDir()
		return filepath.Join(append([]string{home}, elem...)...)
	}
}

func copilotHooksPath() string {
	dir := os.Getenv("COPILOT_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".copilot")
	}
	return filepath.Join(dir, "hooks", "ytta.json")
}

// copilotHooks is the hooks file ytta owns in Copilot CLI's hooks directory,
// which Copilot merges with the user's own files there.
func copilotHooks(bin string, record bool) []byte {
	args := "hook --agent copilot"
	if record {
		args = "hook --record"
	}
	entry := []map[string]any{{"type": "command", "bash": guarded(bin, args), "timeoutSec": 5}}
	hooks := map[string]any{}
	for _, e := range agent.CopilotEvents {
		hooks[e] = entry
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	_ = enc.Encode(map[string]any{"version": 1, "hooks": hooks})
	return b.Bytes()
}

// targetFlags lists the install flags: "--claude, --codex or --gemini".
func targetFlags() string {
	flags := make([]string, len(targets))
	for i, t := range targets {
		flags[i] = "--" + t.name
	}
	last := len(flags) - 1
	return strings.Join(flags[:last], ", ") + " or " + flags[last]
}

func codexHooksPath() string {
	if dir := os.Getenv("CODEX_HOME"); dir != "" {
		return filepath.Join(dir, "hooks.json")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".codex", "hooks.json")
}

// codexEvents leaves out the events the installed Codex does not know: an
// unknown event name could invalidate the hooks file. SessionEnd arrived in
// 0.145 and Interrupt in 0.150.
func codexEvents() (live, recorded []string, note string) {
	live = []string{"SessionStart", "UserPromptSubmit", "PreToolUse", "PermissionRequest", "PostToolUse", "Stop"}
	v := os.Getenv("YTTA_CODEX_VERSION")
	if v == "" {
		out, _ := exec.Command("codex", "--version").Output()
		v = string(out)
	}
	switch {
	case tmuxAtLeast(v, 0, 150):
		live = append(live, "SessionEnd", "Interrupt")
	case tmuxAtLeast(v, 0, 145):
		live = append(live, "SessionEnd")
		note = "this Codex has no Interrupt hook (0.150 adds it): an interrupted turn shows as running until the next prompt"
	case tmuxAtLeast(v, 0, 124):
		note = "this Codex has no SessionEnd or Interrupt hook (0.145 and 0.150 add them): a closed session is forgotten when its pane changes, and an interrupted turn shows as running until the next prompt"
	default:
		note = "no Codex 0.124 or newer found on PATH; installing the events every such version knows. Run this again after upgrading Codex"
	}
	return live, live, note
}

// recordEvents are the hooks the recorder listens to. Every name here must be
// a documented Claude Code event: an unknown one could invalidate the whole
// settings file.
var recordEvents = []string{
	"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse", "PostToolUseFailure",
	"PermissionRequest", "Notification", "Stop", "SubagentStart", "SubagentStop",
	"PreCompact", "SessionEnd",
}

// liveEvents drive the state machine. They run synchronously: the machine
// needs them in order, and the adapter answers in a few milliseconds.
var liveEvents = []string{
	"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse", "PostToolUseFailure",
	"PermissionRequest", "Notification", "Stop", "SessionEnd",
}

// guarded keeps Claude quiet if the plugin is removed without uninstalling:
// a missing binary becomes a no-op instead of an error on every event. The
// trailing comment is how uninstall recognizes ytta's commands, wherever
// the binary lives.
func guarded(bin, args string) string {
	q := shellPath(bin)
	return "test -x " + q + " && " + q + " " + args + "; exit 0 # " + install.Marker
}

// guardedBinary is the binary a guarded or wrapping command tests for, with
// $HOME expanded, or "" when the command has no such test.
func guardedBinary(command string) string {
	_, rest, ok := strings.Cut(command, "test -x ")
	if !ok {
		return ""
	}
	home, _ := os.UserHomeDir()
	var b strings.Builder
	for i := 0; i < len(rest) && rest[i] != ' ' && rest[i] != ';'; i++ {
		switch rest[i] {
		case '\'':
			end := strings.IndexByte(rest[i+1:], '\'')
			if end < 0 {
				return ""
			}
			b.WriteString(rest[i+1 : i+1+end])
			i += end + 1
		case '\\':
			if i+1 < len(rest) {
				i++
				b.WriteByte(rest[i])
			}
		case '"':
			end := strings.IndexByte(rest[i+1:], '"')
			if end < 0 {
				return ""
			}
			b.WriteString(strings.ReplaceAll(rest[i+1:i+1+end], "$HOME", home))
			i += end + 1
		default:
			b.WriteByte(rest[i])
		}
	}
	return b.String()
}

// wrapping is the statusLine command that runs the user's own line through
// ytta. Without the binary it runs their line directly, so removing the
// plugin never blanks a status line.
func wrapping(bin, original64 string) string {
	q := shellPath(bin)
	return "if test -x " + q + "; then " + q + " statusline " + install.WrapFlag + " " + original64 +
		`; else sh -c "$(echo ` + original64 + ` | base64 -d)"; fi # ` + install.Marker
}

func defaultSettingsPath() string {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, "settings.json")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "settings.json")
}

// selfPath refuses a `go run` binary: its temp path disappears once the run
// ends, leaving Claude calling a command that no longer exists.
func selfPath() (string, error) {
	p, err := os.Executable()
	if err != nil {
		return "", err
	}
	if p, err = filepath.EvalSymlinks(p); err != nil {
		return "", err
	}
	if strings.Contains(p, "go-build") {
		return "", errors.New("run a built binary (make build), not go run")
	}
	return p, nil
}

// shellPath writes a path under the home directory as "$HOME"/..., so a
// settings file shared between machines finds the binary wherever home is.
func shellPath(p string) string {
	home, err := os.UserHomeDir()
	if err == nil && home != "" && home != "/" && strings.HasPrefix(p, home+"/") {
		return `"$HOME"` + shellQuote(strings.TrimPrefix(p, home))
	}
	return shellQuote(p)
}

func shellQuote(s string) string {
	if !strings.ContainsAny(s, " '\"$`\\") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func printDiff(before, after []byte) {
	dir, err := os.MkdirTemp("", "ytta-diff")
	if err == nil {
		defer os.RemoveAll(dir)
		a, b := filepath.Join(dir, "current"), filepath.Join(dir, "proposed")
		if os.WriteFile(a, before, 0o600) == nil && os.WriteFile(b, after, 0o600) == nil {
			out, _ := exec.Command("diff", "-u", a, b).Output()
			if len(out) > 0 {
				os.Stdout.Write(out)
				return
			}
		}
	}
	os.Stdout.Write(after)
}

// writeWithBackup keeps the previous file, then replaces it atomically with
// the same permissions so a crash never leaves Claude a half-written file.
// A symlinked file, as dotfiles keep settings, is written at its target so
// the link survives.
func writeWithBackup(path string, before, after []byte) (string, error) {
	if real, err := filepath.EvalSymlinks(path); err == nil {
		path = real
	}
	mode := os.FileMode(0o600)
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}
	var backup string
	if before != nil {
		dir := filepath.Join(store.Root(), "backups")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return "", err
		}
		f, err := os.CreateTemp(dir, "settings-"+time.Now().Format("20060102T150405")+"-*.json")
		if err != nil {
			return "", err
		}
		backup = f.Name()
		defer pruneBackups(dir, 10)
		_, werr := f.Write(before)
		if cerr := f.Close(); werr == nil {
			werr = cerr
		}
		if werr != nil {
			return "", werr
		}
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".settings-*.json")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(after); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Chmod(tmp.Name(), mode); err != nil {
		return "", err
	}
	return backup, os.Rename(tmp.Name(), path)
}

// pruneBackups keeps the newest n settings backups.
func pruneBackups(dir string, n int) {
	files, _ := filepath.Glob(filepath.Join(dir, "settings-*.json"))
	sort.Strings(files) // names start with a timestamp, so this is oldest first
	for len(files) > n {
		_ = os.Remove(files[0])
		files = files[1:]
	}
}

package agent

import (
	"strings"
	"testing"

	"github.com/arisros/ytta/internal/hook"
	"github.com/arisros/ytta/internal/machine"
)

func TestFor(t *testing.T) {
	for name, want := range map[string]string{"": "claude", "claude": "claude", "codex": "codex", "gemini": "gemini", "opencode": "opencode"} {
		if a, ok := For(name); !ok || a.Name != want {
			t.Errorf("For(%q) = %q %v, want %q", name, a.Name, ok, want)
		}
	}
	if _, ok := For("nope"); ok {
		t.Error("an unknown agent was accepted")
	}
}

func TestCodexMap(t *testing.T) {
	cases := []struct {
		in     string
		action hook.Action
		want   machine.Event
	}{
		{`{"hook_event_name":"SessionStart","source":"startup"}`, hook.Begin, nil},
		{`{"hook_event_name":"SessionStart","source":"resume"}`, hook.Begin, nil},
		{`{"hook_event_name":"SessionStart","source":"compact"}`, hook.Ignore, nil},
		{`{"hook_event_name":"SessionEnd","reason":"other"}`, hook.End, nil},
		{`{"hook_event_name":"UserPromptSubmit","prompt":"x"}`, hook.Send, machine.Prompt{At: 7}},
		{`{"hook_event_name":"PreToolUse","tool_name":"Bash"}`, hook.Send, machine.ToolStart{At: 7}},
		{`{"hook_event_name":"PostToolUse","tool_name":"Bash"}`, hook.Send, machine.ToolEnd{At: 7}},
		{`{"hook_event_name":"PermissionRequest","tool_name":"apply_patch"}`, hook.Send, machine.Permission{At: 7, Reason: machine.ReasonPermission, Tool: "apply_patch"}},
		{`{"hook_event_name":"Stop"}`, hook.Send, machine.Stop{At: 7}},
		{`{"hook_event_name":"Interrupt"}`, hook.Send, machine.Interrupt{At: 7}},
		// A subagent's events never move the main turn.
		{`{"hook_event_name":"Stop","agent_id":"a1"}`, hook.Ignore, nil},
		{`{"hook_event_name":"PostToolUse","agent_id":"a1"}`, hook.Ignore, nil},
		{`{"hook_event_name":"SubagentStop"}`, hook.Ignore, nil},
		{`{"hook_event_name":"PreCompact"}`, hook.Ignore, nil},
		// Codex has no notifications; one from elsewhere means nothing here.
		{`{"hook_event_name":"Notification","notification_type":"idle_prompt"}`, hook.Ignore, nil},
	}
	for _, c := range cases {
		p, err := hook.Decode(strings.NewReader(c.in))
		if err != nil {
			t.Fatal(err)
		}
		action, ev := Codex.Map(p, 7)
		if action != c.action || ev != c.want {
			t.Errorf("%s: got %v %#v, want %v %#v", c.in, action, ev, c.action, c.want)
		}
	}
}

// Screens assembled from the strings in Codex's TUI source, not captured
// from a session.
func TestCodexClassify(t *testing.T) {
	cases := map[string]string{
		"• Working (12s • esc to interrupt)\n\n› Ask Codex to do anything\n  ? for shortcuts   100% context left\n":                                     machine.ScreenWorking,
		"  1 background terminal running · /ps to view · /stop to close\n› \n  98% context left\n":                                                      machine.ScreenWorking,
		"Would you like to run the following command?\n  $ rm -rf build\n› 1. Yes, proceed (y)\n  2. No, and tell Codex what to do differently (esc)\n": machine.ScreenDialog,
		"Would you like to make the following edits?\n› 1. Yes, proceed (y)\n":                                                                          machine.ScreenDialog,
		"Do you want to approve network access to \"example.com\"?\n":                                                                                   machine.ScreenDialog,
		"• Ran tests\n\n› Ask Codex to do anything\n  ? for shortcuts   81% context left\n":                                                             machine.ScreenNoDialog,
		"$ ls\nfoo\n": "",
		"":            "",
	}
	for screen, want := range cases {
		if got := Codex.Classify(screen); got != want {
			t.Errorf("classifyCodex(%q) = %q, want %q", screen, got, want)
		}
	}
	for screen := range cases {
		if Codex.Classify(screen) == machine.ScreenIdle {
			t.Errorf("a Codex screen was read as idle, which nothing on it can prove: %q", screen)
		}
	}
}

func mapped(t *testing.T, a Agent, in string) (hook.Action, machine.Event) {
	t.Helper()
	p, err := hook.Decode(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	return a.Map(p, 7)
}

func TestGeminiMap(t *testing.T) {
	cases := []struct {
		in     string
		action hook.Action
		want   machine.Event
	}{
		{`{"hook_event_name":"SessionStart","source":"resume"}`, hook.Begin, nil},
		{`{"hook_event_name":"SessionEnd","reason":"exit"}`, hook.End, nil},
		{`{"hook_event_name":"BeforeAgent","prompt":"x"}`, hook.Send, machine.Prompt{At: 7}},
		{`{"hook_event_name":"BeforeTool","tool_name":"run_shell_command"}`, hook.Send, machine.ToolStart{At: 7}},
		{`{"hook_event_name":"AfterTool","tool_name":"run_shell_command"}`, hook.Send, machine.ToolEnd{At: 7}},
		{`{"hook_event_name":"BeforeModel"}`, hook.Send, machine.ToolEnd{At: 7}},
		{`{"hook_event_name":"Notification","notification_type":"ToolPermission"}`, hook.Send, machine.NeedsInput{At: 7, Reason: machine.ReasonPermission}},
		{`{"hook_event_name":"Notification","notification_type":"Other"}`, hook.Ignore, nil},
		{`{"hook_event_name":"AfterAgent"}`, hook.Send, machine.Stop{At: 7}},
		{`{"hook_event_name":"AfterModel"}`, hook.Ignore, nil},
		{`{"hook_event_name":"PreCompress"}`, hook.Ignore, nil},
		// Claude's names mean nothing to Gemini's mapping.
		{`{"hook_event_name":"UserPromptSubmit"}`, hook.Ignore, nil},
	}
	for _, c := range cases {
		if action, ev := mapped(t, Gemini, c.in); action != c.action || ev != c.want {
			t.Errorf("%s: got %v %#v, want %v %#v", c.in, action, ev, c.action, c.want)
		}
	}
}

func TestOpenCodeMap(t *testing.T) {
	cases := []struct {
		in     string
		action hook.Action
		want   machine.Event
	}{
		{`{"hook_event_name":"SessionStart","source":"startup"}`, hook.Begin, nil},
		{`{"hook_event_name":"SessionEnd"}`, hook.End, nil},
		{`{"hook_event_name":"UserPromptSubmit"}`, hook.Send, machine.Prompt{At: 7}},
		{`{"hook_event_name":"PreToolUse","tool_name":"bash"}`, hook.Send, machine.ToolStart{At: 7}},
		{`{"hook_event_name":"PostToolUse","tool_name":"bash"}`, hook.Send, machine.ToolEnd{At: 7}},
		{`{"hook_event_name":"PermissionRequest","tool_name":"bash"}`, hook.Send, machine.Permission{At: 7, Reason: machine.ReasonPermission, Tool: "bash"}},
		{`{"hook_event_name":"PermissionRequest","tool_name":"AskUserQuestion"}`, hook.Send, machine.Permission{At: 7, Reason: machine.ReasonQuestion}},
		{`{"hook_event_name":"PermissionReplied"}`, hook.Send, machine.ToolEnd{At: 7}},
		{`{"hook_event_name":"Stop"}`, hook.Send, machine.Stop{At: 7}},
		{`{"hook_event_name":"Interrupt"}`, hook.Send, machine.Interrupt{At: 7}},
		{`{"hook_event_name":"Notification","notification_type":"idle_prompt"}`, hook.Ignore, nil},
	}
	for _, c := range cases {
		if action, ev := mapped(t, OpenCode, c.in); action != c.action || ev != c.want {
			t.Errorf("%s: got %v %#v, want %v %#v", c.in, action, ev, c.action, c.want)
		}
	}
}

func TestCopilotMap(t *testing.T) {
	cases := []struct {
		in     string
		action hook.Action
		want   machine.Event
	}{
		{`{"hook_event_name":"SessionStart","source":"resume"}`, hook.Begin, nil},
		{`{"hook_event_name":"SessionEnd","reason":"user_exit"}`, hook.End, nil},
		{`{"hook_event_name":"UserPromptSubmit","prompt":"x"}`, hook.Send, machine.Prompt{At: 7}},
		{`{"hook_event_name":"PreToolUse","tool_name":"bash"}`, hook.Send, machine.ToolStart{At: 7}},
		{`{"hook_event_name":"PostToolUse","tool_name":"bash"}`, hook.Send, machine.ToolEnd{At: 7}},
		{`{"hook_event_name":"PostToolUseFailure","tool_name":"bash"}`, hook.Send, machine.ToolEnd{At: 7}},
		{`{"hook_event_name":"PostToolUse","agent_id":"a1"}`, hook.Ignore, nil},
		{`{"hook_event_name":"Stop","stop_reason":"end_turn"}`, hook.Send, machine.Stop{At: 7}},
		{`{"sessionId":"s","hook_event_name":"Notification","notification_type":"permission_prompt"}`, hook.Send, machine.NeedsInput{At: 7, Reason: machine.ReasonPermission}},
		{`{"hook_event_name":"Notification","notification_type":"elicitation_dialog"}`, hook.Send, machine.NeedsInput{At: 7, Reason: machine.ReasonElicitation}},
		{`{"hook_event_name":"Notification","notification_type":"agent_idle"}`, hook.Ignore, nil},
		{`{"hook_event_name":"Notification","notification_type":"shell_completed"}`, hook.Ignore, nil},
		{`{"hook_event_name":"PermissionRequest","tool_name":"bash"}`, hook.Ignore, nil},
	}
	for _, c := range cases {
		if action, ev := mapped(t, Copilot, c.in); action != c.action || ev != c.want {
			t.Errorf("%s: got %v %#v, want %v %#v", c.in, action, ev, c.action, c.want)
		}
	}
}

func TestDroidMap(t *testing.T) {
	cases := []struct {
		in     string
		action hook.Action
		want   machine.Event
	}{
		{`{"hook_event_name":"SessionStart","source":"startup"}`, hook.Begin, nil},
		{`{"hook_event_name":"SessionStart","source":"compact"}`, hook.Ignore, nil},
		{`{"hook_event_name":"SessionEnd","reason":"prompt_input_exit"}`, hook.End, nil},
		{`{"hook_event_name":"UserPromptSubmit","prompt":"x"}`, hook.Send, machine.Prompt{At: 7}},
		{`{"hook_event_name":"PreToolUse","tool_name":"Execute"}`, hook.Send, machine.ToolStart{At: 7}},
		{`{"hook_event_name":"PostToolUse","tool_name":"Execute"}`, hook.Send, machine.ToolEnd{At: 7}},
		{`{"hook_event_name":"Stop","stop_hook_active":false}`, hook.Send, machine.Stop{At: 7}},
		{`{"hook_event_name":"Notification","notification_type":"permission_prompt"}`, hook.Send, machine.NeedsInput{At: 7, Reason: machine.ReasonPermission}},
		{`{"hook_event_name":"Notification","notification_type":"elicitation_dialog"}`, hook.Send, machine.NeedsInput{At: 7, Reason: machine.ReasonElicitation}},
		{`{"hook_event_name":"Notification","notification_type":"idle_prompt"}`, hook.Send, machine.IdlePrompt{At: 7}},
		{`{"hook_event_name":"Notification","notification_type":"auth_success"}`, hook.Ignore, nil},
		{`{"hook_event_name":"SubagentStop","task_name":"x"}`, hook.Ignore, nil},
	}
	for _, c := range cases {
		if action, ev := mapped(t, Droid, c.in); action != c.action || ev != c.want {
			t.Errorf("%s: got %v %#v, want %v %#v", c.in, action, ev, c.action, c.want)
		}
	}
}

func TestQwenMap(t *testing.T) {
	cases := []struct {
		in     string
		action hook.Action
		want   machine.Event
	}{
		{`{"hook_event_name":"SessionStart","source":"startup"}`, hook.Begin, nil},
		{`{"hook_event_name":"SessionStart","source":"compact"}`, hook.Ignore, nil},
		{`{"hook_event_name":"SessionEnd"}`, hook.End, nil},
		{`{"hook_event_name":"UserPromptSubmit","prompt":"x"}`, hook.Send, machine.Prompt{At: 7}},
		{`{"hook_event_name":"PreToolUse","tool_name":"run_shell_command"}`, hook.Send, machine.ToolStart{At: 7}},
		{`{"hook_event_name":"PostToolUse","agent_id":"a1"}`, hook.Send, machine.ToolEnd{At: 7, Subagent: true}},
		{`{"hook_event_name":"PermissionRequest","tool_name":"run_shell_command"}`, hook.Send, machine.Permission{At: 7, Reason: machine.ReasonPermission, Tool: "run_shell_command"}},
		{`{"hook_event_name":"Notification","notification_type":"permission_prompt"}`, hook.Send, machine.NeedsInput{At: 7, Reason: machine.ReasonPermission}},
		{`{"hook_event_name":"Notification","notification_type":"idle_prompt"}`, hook.Send, machine.IdlePrompt{At: 7}},
		{`{"hook_event_name":"Stop","background_tasks":[{"id":"x"}]}`, hook.Send, machine.Stop{At: 7, Background: 1}},
		{`{"hook_event_name":"StopFailure","error":"rate_limit"}`, hook.Send, machine.Stop{At: 7}},
		{`{"hook_event_name":"PermissionDenied","reason":"classifier_blocked"}`, hook.Ignore, nil},
		{`{"hook_event_name":"MessageDisplay"}`, hook.Ignore, nil},
	}
	for _, c := range cases {
		if action, ev := mapped(t, Qwen, c.in); action != c.action || ev != c.want {
			t.Errorf("%s: got %v %#v, want %v %#v", c.in, action, ev, c.action, c.want)
		}
	}
}

func TestKiloPlugin(t *testing.T) {
	src := KiloPlugin()
	for _, want := range []string{`"--agent", "kilo"`, "const Ytta = async", `export default { id: "ytta", server: Ytta }`, `const YTTA = "__YTTA__"`} {
		if !strings.Contains(src, want) {
			t.Errorf("plugin lacks %q", want)
		}
	}
	for _, not := range []string{"export const Ytta", `"opencode"`} {
		if strings.Contains(src, not) {
			t.Errorf("plugin still has %q", not)
		}
	}
}

func TestKimiMap(t *testing.T) {
	cases := []struct {
		in     string
		action hook.Action
		want   machine.Event
	}{
		{`{"hook_event_name":"SessionStart","source":"resume"}`, hook.Begin, nil},
		{`{"hook_event_name":"SessionEnd"}`, hook.End, nil},
		{`{"hook_event_name":"UserPromptSubmit","prompt":"x"}`, hook.Send, machine.Prompt{At: 7}},
		{`{"hook_event_name":"PreToolUse","tool_name":"Bash"}`, hook.Send, machine.ToolStart{At: 7}},
		{`{"hook_event_name":"PermissionRequest","tool_name":"Bash"}`, hook.Send, machine.Permission{At: 7, Reason: machine.ReasonPermission, Tool: "Bash"}},
		{`{"hook_event_name":"PermissionResult"}`, hook.Send, machine.ToolEnd{At: 7}},
		{`{"hook_event_name":"Stop"}`, hook.Send, machine.Stop{At: 7}},
		{`{"hook_event_name":"StopFailure"}`, hook.Send, machine.Stop{At: 7}},
		{`{"hook_event_name":"Interrupt"}`, hook.Send, machine.Interrupt{At: 7}},
		{`{"hook_event_name":"Notification"}`, hook.Ignore, nil},
		{`{"hook_event_name":"TurnStarted"}`, hook.Ignore, nil},
		{`{"hook_event_name":"SubagentStop"}`, hook.Ignore, nil},
	}
	for _, c := range cases {
		if action, ev := mapped(t, Kimi, c.in); action != c.action || ev != c.want {
			t.Errorf("%s: got %v %#v, want %v %#v", c.in, action, ev, c.action, c.want)
		}
	}
}

func TestHermesMap(t *testing.T) {
	cases := []struct {
		in     string
		action hook.Action
		want   machine.Event
	}{
		{`{"hook_event_name":"on_session_start","session_id":"s","tool_name":null}`, hook.Begin, nil},
		{`{"hook_event_name":"on_session_finalize"}`, hook.End, nil},
		{`{"hook_event_name":"pre_llm_call","extra":{"user_message":"x"}}`, hook.Send, machine.Prompt{At: 7}},
		{`{"hook_event_name":"pre_tool_call","tool_name":"terminal","tool_input":{"command":"x"}}`, hook.Send, machine.ToolStart{At: 7}},
		{`{"hook_event_name":"post_tool_call","tool_name":"terminal"}`, hook.Send, machine.ToolEnd{At: 7}},
		{`{"hook_event_name":"pre_approval_request","tool_name":null,"session_id":null}`, hook.Send, machine.Permission{At: 7, Reason: machine.ReasonPermission}},
		{`{"hook_event_name":"post_approval_response","extra":{"choice":"deny"}}`, hook.Send, machine.ToolEnd{At: 7}},
		{`{"hook_event_name":"on_session_end","extra":{"interrupted":true}}`, hook.Send, machine.Stop{At: 7}},
		{`{"hook_event_name":"post_llm_call"}`, hook.Ignore, nil},
		{`{"hook_event_name":"subagent_stop"}`, hook.Ignore, nil},
	}
	for _, c := range cases {
		if action, ev := mapped(t, Hermes, c.in); action != c.action || ev != c.want {
			t.Errorf("%s: got %v %#v, want %v %#v", c.in, action, ev, c.action, c.want)
		}
	}
}

func TestScreenWithoutAClassifier(t *testing.T) {
	if got := (Agent{Name: "quiet"}).Screen("anything"); got != "" {
		t.Errorf("Screen = %q", got)
	}
}

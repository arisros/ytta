package agent

import (
	"strings"

	"github.com/arisros/ytta/internal/hook"
	"github.com/arisros/ytta/internal/machine"
)

// Agy is Google's Antigravity CLI (agy). Its hooks live under a named entry in
// ~/.gemini/config/hooks.json and send camelCase payloads with conversationId,
// toolCall.name, terminationReason and fullyIdle.
//
// Transitions are verified against recordings from agy v1.5.18: ask_question
// enters waiting through PreToolUse and resolves on PostToolUse, while Esc
// mid-turn fires no Stop hook and prints an Interrupted line above the input
// box that the screen reader clears. Permission dialogs have no dedicated hook
// yet and are picked up from the screen on reconcile and view refresh. There
// is no SessionEnd hook; ytta forgets the pane when its foreground command
// leaves agy.
var Agy = Agent{
	Name:     "agy",
	Map:      mapAgy,
	Classify: classifyAgy,
	Evidence: agyLastLine,
	Detect:   isAgy,
}

// AgyFlatEvents are the Antigravity hook events configured as flat handler lists.
var AgyFlatEvents = []string{"SessionStart", "PreInvocation", "Stop"}

// AgyToolEvents are the Antigravity hook events configured with a tool matcher.
var AgyToolEvents = []string{"PreToolUse", "PostToolUse"}

func isAgy(command string) bool { return command == "agy" }

func mapAgy(p hook.Payload, now int64) (hook.Action, machine.Event) {
	switch p.Event {
	case "SessionStart":
		return hook.Begin, nil
	case "PreInvocation":
		return hook.Send, machine.Prompt{At: now}
	case "PreToolUse":
		if p.ToolName == "ask_question" || p.ToolName == "ask_user_question" {
			return hook.Send, machine.Permission{At: now, Reason: machine.ReasonQuestion}
		}
		return hook.Send, machine.ToolStart{At: now}
	case "PostToolUse":
		return hook.Send, machine.ToolEnd{At: now}
	case "Stop":
		if p.TerminationReason == "USER_CANCELED" {
			return hook.Send, machine.Interrupt{At: now}
		}
		bg := 0
		if p.FullyIdle != nil && !*p.FullyIdle {
			bg = 1
		}
		return hook.Send, machine.Stop{At: now, Background: bg}
	}
	return hook.Ignore, nil
}

// classifyAgy reads an Antigravity CLI screen using only explicit markers
// observed in agy's TUI. Dialog markers are checked before "esc to cancel"
// because agy keeps that footer visible while an interactive prompt is open.
// Idle requires agy's own Interrupted notice right above the input box.
func classifyAgy(screen string) string {
	lines := strings.Split(strings.TrimRight(screen, "\n "), "\n")
	var tail []string
	for i := len(lines) - 1; i >= 0 && len(tail) < 30; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			tail = append(tail, lines[i])
		}
	}
	has := func(words ...string) bool {
		for _, l := range tail {
			for _, w := range words {
				if strings.Contains(l, w) {
					return true
				}
			}
		}
		return false
	}
	switch {
	case has("↑/↓ Navigate · ", "Requesting permission for:", "Run this command?", "Do you trust the contents of this project?"):
		return machine.ScreenDialog
	case has("esc to cancel"):
		return machine.ScreenWorking
	}
	if !has("? for shortcuts", "shift+tab to cycle") {
		return ""
	}
	if strings.Contains(agyLastLine(screen), "Interrupted") {
		return machine.ScreenIdle
	}
	return machine.ScreenNoDialog
}

// agyLastLine returns the first transcript line above agy's "> " input box,
// skipping horizontal rule separators.
func agyLastLine(screen string) string {
	lines := strings.Split(strings.TrimRight(screen, "\n "), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		t := strings.TrimSpace(lines[i])
		if strings.HasPrefix(t, ">") && i > 0 {
			return lastTranscriptLine(reverse(lines[:i]))
		}
	}
	return ""
}

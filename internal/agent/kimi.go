package agent

import (
	"github.com/arisros/ytta/internal/hook"
	"github.com/arisros/ytta/internal/machine"
)

// Kimi is Kimi Code CLI. Its hooks send Claude Code's payload fields and
// mostly its event names, and report what Claude Code does not: the answer
// to a permission prompt, and Esc.
//
// The mapping comes from Kimi Code's hooks reference, not yet from recorded
// sessions, and ytta reads no Kimi screens.
var Kimi = Agent{
	Name: "kimi",
	Map:  mapKimi,
}

// KimiEvents are the hook names ytta registers with Kimi Code CLI.
var KimiEvents = []string{
	"SessionStart", "SessionEnd", "UserPromptSubmit", "PreToolUse", "PermissionRequest", "PermissionResult",
	"Stop", "StopFailure", "Interrupt",
}

func mapKimi(p hook.Payload, now int64) (hook.Action, machine.Event) {
	switch p.Event {
	case "SessionStart":
		return hook.Begin, nil
	case "SessionEnd":
		return hook.End, nil
	case "UserPromptSubmit":
		return hook.Send, machine.Prompt{At: now}
	case "PreToolUse":
		return hook.Send, machine.ToolStart{At: now}
	case "PermissionRequest":
		return hook.Send, machine.Permission{At: now, Reason: machine.ReasonPermission, Tool: hook.ToolClass(p.ToolName)}
	case "PermissionResult":
		return hook.Send, machine.ToolEnd{At: now}
	case "Stop", "StopFailure":
		return hook.Send, machine.Stop{At: now}
	case "Interrupt":
		return hook.Send, machine.Interrupt{At: now}
	}
	return hook.Ignore, nil
}

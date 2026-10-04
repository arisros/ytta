package agent

import (
	"github.com/arisros/ytta/internal/hook"
	"github.com/arisros/ytta/internal/machine"
)

// Droid is Factory's Droid. Its hooks have Claude Code's shape, payload
// fields and event names, without a PermissionRequest: a wait is a
// notification. A cancelled turn sends a notification instead of a Stop.
//
// The mapping comes from Factory's hooks reference, not yet from recorded
// sessions, and ytta reads no Droid screens.
var Droid = Agent{
	Name: "droid",
	Map:  mapDroid,
}

// DroidEvents are the hook names ytta registers with Droid.
var DroidEvents = []string{
	"SessionStart", "SessionEnd", "UserPromptSubmit", "PreToolUse", "PostToolUse", "Notification", "Stop",
}

func mapDroid(p hook.Payload, now int64) (hook.Action, machine.Event) {
	switch p.Event {
	case "SessionStart":
		if p.Source == "compact" {
			return hook.Ignore, nil
		}
		return hook.Begin, nil
	case "SessionEnd":
		return hook.End, nil
	case "UserPromptSubmit":
		return hook.Send, machine.Prompt{At: now}
	case "PreToolUse":
		return hook.Send, machine.ToolStart{At: now}
	case "PostToolUse":
		return hook.Send, machine.ToolEnd{At: now}
	case "Stop":
		return hook.Send, machine.Stop{At: now}
	case "Notification":
		switch p.NotificationType {
		case "permission_prompt":
			return hook.Send, machine.NeedsInput{At: now, Reason: machine.ReasonPermission}
		case "elicitation_dialog":
			return hook.Send, machine.NeedsInput{At: now, Reason: machine.ReasonElicitation}
		case "idle_prompt":
			return hook.Send, machine.IdlePrompt{At: now}
		}
	}
	return hook.Ignore, nil
}

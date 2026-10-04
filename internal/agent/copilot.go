package agent

import (
	"github.com/arisros/ytta/internal/hook"
	"github.com/arisros/ytta/internal/machine"
)

// Copilot is GitHub Copilot CLI. Registered under its PascalCase event
// names, its hooks send Claude Code's payload fields. A wait is reported
// only by a notification: its permissionRequest hook runs before the rules
// that decide whether to ask, so it also fires for a tool nobody is asked
// about, and nothing reports the answer but the tool's own end.
//
// The mapping comes from Copilot CLI's hooks reference, not yet from
// recorded sessions, and ytta reads no Copilot screens.
var Copilot = Agent{
	Name: "copilot",
	Map:  mapCopilot,
}

// CopilotEvents are the hook names ytta registers with Copilot CLI.
var CopilotEvents = []string{
	"SessionStart", "SessionEnd", "UserPromptSubmit", "PreToolUse", "PostToolUse", "PostToolUseFailure", "Stop", "notification",
}

func mapCopilot(p hook.Payload, now int64) (hook.Action, machine.Event) {
	// A subagent's events say nothing about the turn the user is waiting on.
	if p.AgentID != "" {
		return hook.Ignore, nil
	}
	switch p.Event {
	case "SessionStart":
		return hook.Begin, nil
	case "SessionEnd":
		return hook.End, nil
	case "UserPromptSubmit":
		return hook.Send, machine.Prompt{At: now}
	case "PreToolUse":
		return hook.Send, machine.ToolStart{At: now}
	case "PostToolUse", "PostToolUseFailure":
		return hook.Send, machine.ToolEnd{At: now}
	case "Stop":
		return hook.Send, machine.Stop{At: now}
	case "Notification":
		switch p.NotificationType {
		case "permission_prompt":
			return hook.Send, machine.NeedsInput{At: now, Reason: machine.ReasonPermission}
		case "elicitation_dialog":
			return hook.Send, machine.NeedsInput{At: now, Reason: machine.ReasonElicitation}
		}
	}
	return hook.Ignore, nil
}

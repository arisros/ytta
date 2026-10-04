package agent

import (
	"github.com/arisros/ytta/internal/hook"
	"github.com/arisros/ytta/internal/machine"
)

// Qwen is Qwen Code. Its hooks carry Claude Code's event names and payload
// fields, so Claude's mapping applies. It adds StopFailure, a turn the API
// ended with an error.
//
// The mapping comes from Qwen Code's hooks reference, not yet from recorded
// sessions, and ytta reads no Qwen screens.
var Qwen = Agent{
	Name: "qwen",
	Map:  mapQwen,
}

// QwenEvents are the hook names ytta registers with Qwen Code.
var QwenEvents = []string{
	"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse", "PostToolUseFailure",
	"PermissionRequest", "Notification", "Stop", "StopFailure", "SessionEnd",
}

func mapQwen(p hook.Payload, now int64) (hook.Action, machine.Event) {
	if p.Event == "StopFailure" {
		return hook.Send, machine.Stop{At: now}
	}
	return hook.Map(p, now)
}

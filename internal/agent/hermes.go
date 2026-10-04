package agent

import (
	"github.com/arisros/ytta/internal/hook"
	"github.com/arisros/ytta/internal/machine"
)

// Hermes is Hermes Agent. Its shell hooks send hook_event_name and
// session_id as Claude Code's do, under its own event names. What it calls
// the end of a session is the end of each turn, interrupted or not; the
// session itself ends at on_session_finalize.
//
// The mapping comes from Hermes Agent's hooks reference, not yet from
// recorded sessions, and ytta reads no Hermes screens.
var Hermes = Agent{
	Name: "hermes",
	Map:  mapHermes,
}

// HermesEvents are the hook names ytta registers with Hermes Agent.
var HermesEvents = []string{
	"on_session_start", "pre_llm_call", "pre_tool_call", "post_tool_call",
	"pre_approval_request", "post_approval_response", "on_session_end", "on_session_finalize",
}

func mapHermes(p hook.Payload, now int64) (hook.Action, machine.Event) {
	switch p.Event {
	case "on_session_start":
		return hook.Begin, nil
	case "on_session_finalize":
		return hook.End, nil
	case "pre_llm_call":
		return hook.Send, machine.Prompt{At: now}
	case "pre_tool_call":
		return hook.Send, machine.ToolStart{At: now}
	case "post_tool_call", "post_approval_response":
		return hook.Send, machine.ToolEnd{At: now}
	case "pre_approval_request":
		return hook.Send, machine.Permission{At: now, Reason: machine.ReasonPermission, Tool: hook.ToolClass(p.ToolName)}
	case "on_session_end":
		return hook.Send, machine.Stop{At: now}
	}
	return hook.Ignore, nil
}

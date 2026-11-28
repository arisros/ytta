// Package record captures a redacted trace of Claude Code hook events.
//
// The trace exists to learn real event sequences (interrupts, permission
// approvals and denials, subagents) that the hook documentation leaves open.
// It keeps event metadata only: no prompt, tool input, path, title or raw
// session id ever leaves the hook payload.
package record

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/arisros/ytta/internal/hook"
	"github.com/arisros/ytta/internal/store"
)

// Entry is one redacted hook event.
type Entry struct {
	TS                int64    `json:"ts"`
	Event             string   `json:"event"`
	Session           string   `json:"session,omitempty"`
	Pane              string   `json:"pane,omitempty"`
	NotificationType  string   `json:"notification_type,omitempty"`
	Tool              string   `json:"tool,omitempty"`
	Source            string   `json:"source,omitempty"`
	Reason            string   `json:"reason,omitempty"`
	AgentType         string   `json:"agent_type,omitempty"`
	StopHookActive    *bool    `json:"stop_hook_active,omitempty"`
	BackgroundTasks   *int     `json:"background_tasks,omitempty"`
	SessionCrons      *int     `json:"session_crons,omitempty"`
	TerminationReason string   `json:"termination_reason,omitempty"`
	FullyIdle         *bool    `json:"fully_idle,omitempty"`
	Keys              []string `json:"keys"`
}

type payload struct {
	HookEventName    string `json:"hook_event_name"`
	HookEventCamel   string `json:"hookEventName"`
	SessionID        string `json:"session_id"`
	SessionCamel     string `json:"sessionId"`
	ConversationID   string `json:"conversationId"`
	NotificationType string `json:"notification_type"`
	ToolName         string `json:"tool_name"`
	ToolNameCamel    string `json:"toolName"`
	ToolCall         struct {
		Name string `json:"name"`
	} `json:"toolCall"`
	Source            string `json:"source"`
	Reason            string `json:"reason"`
	AgentType         string `json:"agent_type"`
	StopHookActive    *bool  `json:"stop_hook_active"`
	TerminationReason string `json:"terminationReason"`
	FullyIdle         *bool  `json:"fullyIdle"`
}

// FromHook reads a hook payload and returns its redacted entry. Keys lists the
// payload's top-level field names, without values, so undocumented payload
// shapes can be learned safely.
func FromHook(r io.Reader, pane string, now time.Time) (Entry, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return Entry{}, err
	}
	var p payload
	if err := json.Unmarshal(raw, &p); err != nil {
		return Entry{}, fmt.Errorf("decode hook payload: %w", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return Entry{}, fmt.Errorf("decode hook payload: %w", err)
	}
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	event := firstNonEmpty(p.HookEventName, p.HookEventCamel)
	session := firstNonEmpty(p.SessionID, p.SessionCamel, p.ConversationID)
	tool := firstNonEmpty(p.ToolName, p.ToolNameCamel, p.ToolCall.Name)

	return Entry{
		TS:                now.UnixMilli(),
		Event:             event,
		Session:           shortHash(session),
		Pane:              pane,
		NotificationType:  p.NotificationType,
		Tool:              hook.ToolClass(tool),
		Source:            p.Source,
		Reason:            p.Reason,
		AgentType:         p.AgentType,
		StopHookActive:    p.StopHookActive,
		BackgroundTasks:   hook.CountPtr(fields["background_tasks"]),
		SessionCrons:      hook.CountPtr(fields["session_crons"]),
		TerminationReason: p.TerminationReason,
		FullyIdle:         p.FullyIdle,
		Keys:              keys,
	}, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// Append writes the entry as one JSON line to a dated file under dir. A single
// O_APPEND write keeps concurrent hooks from interleaving lines.
func Append(dir string, e Entry) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	line, err := json.Marshal(e)
	if err != nil {
		return err
	}
	name := filepath.Join(dir, time.UnixMilli(e.TS).Format("2006-01-02")+".jsonl")
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return err
	}
	_, werr := f.Write(append(line, '\n'))
	cerr := f.Close()
	if werr != nil {
		return werr
	}
	return cerr
}

// DefaultDir is where traces are kept: local state, never inside the repo.
func DefaultDir() string { return filepath.Join(store.Root(), "record") }

func shortHash(s string) string {
	if s == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])[:12]
}

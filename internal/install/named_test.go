package install

import (
	"encoding/json"
	"strings"
	"testing"
)

const agyHooks = `{
  "lint-checker": {
    "PostToolUse": [
      {
        "matcher": "run_command",
        "hooks": [
          {
            "type": "command",
            "command": "echo a && echo b",
            "timeout": 10
          }
        ]
      }
    ]
  },
  "reminder": {
    "PreInvocation": [
      {
        "type": "command",
        "command": "./scripts/reminder.sh"
      }
    ]
  }
}
`

func TestAddNamedThenRemoveNamedRoundTrips(t *testing.T) {
	h := Hook{Command: `test -x /opt/ytta && /opt/ytta hook --agent agy; exit 0 # ytta`, Timeout: 5}
	flat := []string{"SessionStart", "PreInvocation", "Stop"}
	tools := []string{"PreToolUse", "PostToolUse"}

	added, err := AddNamed([]byte(agyHooks), Marker, flat, tools, h)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(added), `\u0026`) || !strings.Contains(string(added), "&&") {
		t.Error("&& was escaped")
	}
	// Key order of foreign entries is preserved.
	if strings.Index(string(added), `"lint-checker"`) > strings.Index(string(added), `"reminder"`) {
		t.Error("existing key order changed")
	}

	// Verify flat vs matcher-grouped layout.
	var parsed map[string]map[string]json.RawMessage
	if err := json.Unmarshal(added, &parsed); err != nil {
		t.Fatal(err)
	}
	entry := parsed[Marker]
	for _, ev := range flat {
		var items []struct {
			Type, Command string
			Timeout       int
		}
		if err := json.Unmarshal(entry[ev], &items); err != nil || len(items) != 1 || items[0].Command != h.Command || items[0].Timeout != 5 {
			t.Errorf("flat event %s: %s (err %v)", ev, entry[ev], err)
		}
	}
	for _, ev := range tools {
		var groups []struct {
			Matcher string
			Hooks   []struct {
				Type, Command string
				Timeout       int
			}
		}
		if err := json.Unmarshal(entry[ev], &groups); err != nil || len(groups) != 1 || groups[0].Matcher != "*" ||
			len(groups[0].Hooks) != 1 || groups[0].Hooks[0].Command != h.Command {
			t.Errorf("tool event %s: %s (err %v)", ev, entry[ev], err)
		}
	}

	events, err := NamedOwned(added, Marker)
	if err != nil || len(events) != len(flat)+len(tools) {
		t.Errorf("NamedOwned = %v, err %v", events, err)
	}
	cmds, err := NamedCommands(added, Marker)
	if err != nil || len(cmds) != len(flat)+len(tools) {
		t.Errorf("NamedCommands = %v, err %v", cmds, err)
	}

	// Idempotent.
	again, err := AddNamed(added, Marker, flat, tools, h)
	if err != nil {
		t.Fatal(err)
	}
	if string(again) != string(added) {
		t.Errorf("second AddNamed changed the file:\n%s", again)
	}

	// Round trip restores original byte-for-byte.
	removed, err := RemoveNamed(added, Marker)
	if err != nil {
		t.Fatal(err)
	}
	if string(removed) != agyHooks {
		t.Errorf("round trip changed the file:\n%s", removed)
	}
}

func TestAddNamedRefusesForeignEntryWithSameName(t *testing.T) {
	foreign := `{"ytta": {"Stop": [{"type": "command", "command": "echo foreign"}]}}`
	h := Hook{Command: "/opt/ytta hook --agent agy", Timeout: 5}
	if _, err := AddNamed([]byte(foreign), Marker, []string{"Stop"}, nil, h); err == nil {
		t.Fatal("overwrote foreign entry")
	}
	if _, err := RemoveNamed([]byte(foreign), Marker); err == nil {
		t.Fatal("removed foreign entry")
	}
}

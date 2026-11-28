package install

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// AddNamed registers a top-level named hook object (as Antigravity CLI's
// hooks.json uses), replacing any earlier entry under name while keeping every
// other key and its order intact.
//
// Events in flatEvents hold handler objects directly; events in toolEvents wrap
// handlers in a {"matcher":"*","hooks":[...]} group.
func AddNamed(settings []byte, name string, flatEvents, toolEvents []string, h Hook) ([]byte, error) {
	if !strings.Contains(h.Command, Marker) {
		return nil, fmt.Errorf("command %q does not contain marker %q", h.Command, Marker)
	}
	cleaned, err := RemoveNamed(settings, name)
	if err != nil {
		return nil, err
	}
	root, err := parseObject(cleaned)
	if err != nil {
		return nil, err
	}

	entry := object{{"type", mustJSON("command")}, {"command", mustJSON(h.Command)}}
	if h.Timeout > 0 {
		entry = append(entry, member{"timeout", mustJSON(h.Timeout)})
	}
	flat := mustJSON([]json.RawMessage{entry.raw()})
	grouped := mustJSON([]json.RawMessage{
		object{{"matcher", mustJSON("*")}, {"hooks", flat}}.raw(),
	})

	block := object{}
	for _, ev := range flatEvents {
		block = block.set(ev, flat)
	}
	for _, ev := range toolEvents {
		block = block.set(ev, grouped)
	}
	root = root.set(name, block.raw())
	return format(root.raw())
}

// RemoveNamed deletes the top-level named hook entry if ytta owns it, leaving
// every other key and value untouched.
func RemoveNamed(settings []byte, name string) ([]byte, error) {
	if len(bytes.TrimSpace(settings)) == 0 {
		settings = []byte("{}")
	}
	root, err := parseObject(settings)
	if err != nil {
		return nil, err
	}
	raw, ok := root.get(name)
	if !ok {
		return format(root.raw())
	}
	if !owned(string(raw)) {
		return nil, fmt.Errorf("%s already exists and is not owned by %s", name, Marker)
	}
	kept := object{}
	for _, m := range root {
		if m.Key != name {
			kept = append(kept, m)
		}
	}
	return format(kept.raw())
}

// NamedOwned reports which events under top-level name currently carry a ytta hook.
func NamedOwned(settings []byte, name string) ([]string, error) {
	root, err := parseObject(settings)
	if err != nil {
		return nil, err
	}
	raw, ok := root.get(name)
	if !ok {
		return nil, nil
	}
	block, err := parseObject(raw)
	if err != nil {
		return nil, err
	}
	var events []string
	for _, ev := range block {
		if bytes.Contains(ev.Value, []byte(Marker)) {
			events = append(events, ev.Key)
		}
	}
	return events, nil
}

// NamedCommands returns the command of every ytta hook entry under top-level
// name, handling both flat handler lists and matcher-grouped lists.
func NamedCommands(settings []byte, name string) ([]string, error) {
	root, err := parseObject(settings)
	if err != nil {
		return nil, err
	}
	raw, ok := root.get(name)
	if !ok {
		return nil, nil
	}
	block, err := parseObject(raw)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, ev := range block {
		var items []struct {
			Command string `json:"command"`
			Hooks   []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		}
		if err := json.Unmarshal(ev.Value, &items); err != nil {
			continue
		}
		for _, it := range items {
			if strings.Contains(it.Command, Marker) {
				out = append(out, it.Command)
			}
			for _, h := range it.Hooks {
				if strings.Contains(h.Command, Marker) {
					out = append(out, h.Command)
				}
			}
		}
	}
	return out, nil
}

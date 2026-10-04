package agent

import (
	_ "embed" // the extension source below
)

// Pi is the Pi coding agent. It has no hook commands: an extension file
// inside Pi listens to its events and runs `ytta hook` for each, with a
// payload in the shape the other agents send. Pi asks no permissions, so
// it is never waiting: it runs, it is done, or it has ended.
//
// The extension comes from Pi's extension API and event types, not yet from
// recorded sessions, and ytta reads no Pi screens.
var Pi = Agent{
	Name: "pi",
	Map:  mapOpenCode,
}

// PiExtension is the extension's source. __YTTA__ stands for the path of
// the ytta binary, filled in when it is installed.
//
//go:embed pi-extension.js
var PiExtension string

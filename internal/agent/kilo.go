package agent

import "strings"

// Kilo is Kilo Code CLI, which carries opencode's plugin interface: the same
// events reach ytta through the same plugin, loaded from Kilo's own plugin
// directory and exported the way Kilo expects.
//
// Nothing here comes from recorded sessions yet, and ytta reads no Kilo
// screens.
var Kilo = Agent{
	Name: "kilo",
	Map:  mapOpenCode,
}

// KiloPlugin is the opencode plugin's source as Kilo loads it. __YTTA__
// stands for the path of the ytta binary.
func KiloPlugin() string {
	return strings.NewReplacer(
		`"--agent", "opencode"`, `"--agent", "kilo"`,
		"export const Ytta = async", "const Ytta = async",
		"this opencode's sessions", "this Kilo's sessions",
		"--opencode", "--kilo",
	).Replace(OpenCodePlugin) + "\nexport default { id: \"ytta\", server: Ytta }\n"
}

package install

import "strings"

const (
	blockBegin = "# " + Marker + " begin"
	blockEnd   = "# " + Marker + " end"
)

// AddBlock puts body at the end of a TOML or YAML file between two comment
// lines, replacing an earlier block. Everything outside the block is kept
// byte for byte.
func AddBlock(file []byte, body string) []byte {
	out := string(RemoveBlock(file))
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return []byte(out + blockBegin + "\n" + strings.TrimRight(body, "\n") + "\n" + blockEnd + "\n")
}

// RemoveBlock deletes the block AddBlock wrote, and nothing else.
func RemoveBlock(file []byte) []byte {
	s := string(file)
	start := strings.Index(s, blockBegin+"\n")
	if start < 0 || (start > 0 && s[start-1] != '\n') {
		return file
	}
	end := strings.Index(s[start:], blockEnd+"\n")
	if end < 0 {
		return file
	}
	return []byte(s[:start] + s[start+end+len(blockEnd)+1:])
}

// HasBlock reports whether file carries a block AddBlock wrote.
func HasBlock(file []byte) bool {
	return len(RemoveBlock(file)) != len(file)
}

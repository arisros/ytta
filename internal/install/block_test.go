package install

import "testing"

const toml = "model = \"k2\"\n\n[[hooks]]\nevent = \"Stop\"\ncommand = \"say done\"\n"

func TestBlockRoundTrips(t *testing.T) {
	added := AddBlock([]byte(toml), "[[hooks]]\nevent = \"Stop\"\ncommand = \"ytta hook\"")
	if !HasBlock(added) {
		t.Fatal("block not found after AddBlock")
	}
	if got := string(RemoveBlock(added)); got != toml {
		t.Errorf("round trip changed the file:\n%s", got)
	}
	if HasBlock([]byte(toml)) {
		t.Error("a file without the block reports one")
	}
}

func TestAddBlockIsIdempotentAndReplaces(t *testing.T) {
	once := AddBlock([]byte(toml), "a = 1")
	if twice := AddBlock(once, "a = 1"); string(twice) != string(once) {
		t.Errorf("second AddBlock changed the file:\n%s", twice)
	}
	replaced := string(AddBlock(once, "b = 2"))
	if want := toml + "# ytta begin\nb = 2\n# ytta end\n"; replaced != want {
		t.Errorf("got:\n%s\nwant:\n%s", replaced, want)
	}
}

func TestAddBlockToEmptyOrUnterminatedFile(t *testing.T) {
	if got, want := string(AddBlock(nil, "a = 1")), "# ytta begin\na = 1\n# ytta end\n"; got != want {
		t.Errorf("empty file: got %q", got)
	}
	if got, want := string(AddBlock([]byte("x = 1"), "a = 1")), "x = 1\n# ytta begin\na = 1\n# ytta end\n"; got != want {
		t.Errorf("unterminated file: got %q", got)
	}
}

func TestRemoveBlockLeavesAnUnclosedBlockAlone(t *testing.T) {
	in := "x = 1\n# ytta begin\na = 1\n"
	if got := string(RemoveBlock([]byte(in))); got != in {
		t.Errorf("got %q", got)
	}
}

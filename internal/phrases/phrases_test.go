package phrases

import "testing"

// A footnote is printed as "* <line>" inside a 46 column box.
func TestFootnotesFitTheLabel(t *testing.T) {
	all := Footnotes()
	if len(all) < 5 {
		t.Fatalf("only %d footnotes", len(all))
	}
	for _, f := range all {
		if len(f) > 44 {
			t.Errorf("footnote is %d chars, max 44: %q", len(f), f)
		}
	}
}

func TestFootnoteIsDeterministic(t *testing.T) {
	if Footnote("sha256:abc") != Footnote("sha256:abc") {
		t.Error("same seed gave different footnotes")
	}
}

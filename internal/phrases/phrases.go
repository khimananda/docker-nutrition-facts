// Package phrases holds the jokes. Contributors add lines to the .txt files.
package phrases

import (
	_ "embed"
	"hash/fnv"
	"strings"
)

//go:embed footnotes.txt
var footnotesRaw string

// Footnotes returns every footnote line.
func Footnotes() []string { return lines(footnotesRaw) }

// Footnote picks a footnote deterministically from a seed, so the same
// image always gets the same joke.
func Footnote(seed string) string {
	all := Footnotes()
	if len(all) == 0 {
		return ""
	}
	h := fnv.New32a()
	h.Write([]byte(seed))
	return all[int(h.Sum32()%uint32(len(all)))]
}

func lines(raw string) []string {
	var out []string
	for _, l := range strings.Split(raw, "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		out = append(out, l)
	}
	return out
}

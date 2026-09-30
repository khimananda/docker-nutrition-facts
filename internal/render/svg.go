package render

import (
	"fmt"
	"html"
	"strings"

	"github.com/khimananda/docker-nutrition-facts/internal/facts"
)

// SVG renders an FDA-style label: black on white, Helvetica, heavy rules.
// It deliberately ignores dark mode, like a real food label.
func SVG(r facts.Report) string {
	const (
		w      = 380
		margin = 12
		inner  = w - 2*margin
	)
	var body strings.Builder
	y := margin
	text := func(x, y int, anchor, weight string, size int, fill, s string) {
		fmt.Fprintf(&body, `  <text x="%d" y="%d" text-anchor="%s" font-weight="%s" font-size="%d" fill="%s">%s</text>`+"\n",
			x, y, anchor, weight, size, fill, html.EscapeString(s))
	}
	rule := func(h int) {
		fmt.Fprintf(&body, `  <rect x="%d" y="%d" width="%d" height="%d" fill="#000"/>`+"\n", margin, y, inner, h)
		y += h + 4
	}
	for _, row := range Rows(r) {
		switch row.Kind {
		case Title:
			y += 34
			text(margin, y, "start", "900", 36, "#000", row.Label)
			y += 8
		case Sub:
			y += 18
			text(margin, y, "start", "400", 14, "#000", fit(row.Label, 48))
			y += 4
		case Thick:
			y += 2
			rule(8)
		case Thin:
			y += 2
			rule(1)
		case Header:
			y += 14
			text(margin, y, "start", "700", 12, "#000", row.Label)
			text(w-margin, y, "end", "700", 12, "#000", row.DV)
			y += 4
		case Fact:
			y += 18
			weight := "400"
			x := margin
			if row.Bold && !row.Indent {
				weight = "700"
			}
			if row.Indent {
				x += 16
			}
			fill := "#000"
			if row.Bad {
				fill = "#c1121f"
			}
			text(x, y, "start", weight, 14, "#000", row.Label)
			text(w-margin-58, y, "end", "400", 14, fill, row.Amount)
			text(w-margin, y, "end", "700", 14, "#000", row.DV)
			y += 5
			fmt.Fprintf(&body, `  <rect x="%d" y="%d" width="%d" height="1" fill="#000"/>`+"\n", margin, y, inner)
		case GradeRow:
			y += 26
			text(margin, y, "start", "900", 22, gradeColor(r.Grade), row.Label)
			text(w-margin, y, "end", "700", 14, "#000", row.Amount)
			y += 6
		case Note:
			y += 16
			weight, size, fill := "400", 11, "#000"
			if row.Bold {
				weight, size = "700", 13
			}
			if row.Bad {
				fill = "#c1121f"
			}
			text(margin, y, "start", weight, size, fill, fit(row.Label, 56))
		}
	}
	h := y + margin + 4
	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" font-family="Helvetica, Arial, sans-serif">
  <title>Nutrition Facts for %s</title>
  <rect x="1" y="1" width="%d" height="%d" fill="#fff" stroke="#000" stroke-width="2"/>
%s</svg>
`, w, h, w, h, html.EscapeString(ShortName(r.Ref)), w-2, h-2, body.String())
}

func gradeColor(g string) string {
	switch g {
	case "A", "B":
		return "#1b7f3b"
	case "C":
		return "#b07d00"
	case "D":
		return "#d35400"
	}
	return "#c1121f"
}

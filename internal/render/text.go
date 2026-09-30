package render

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/khimananda/docker-nutrition-facts/internal/facts"
)

// Width is the inner width of the text label.
const Width = 46

var (
	boldStyle  = lipgloss.NewStyle().Bold(true)
	badStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	gradeStyle = map[string]lipgloss.Style{
		"A": lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true),
		"B": lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true),
		"C": lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true),
		"D": lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true),
		"F": lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true),
	}
)

// Text draws the boxed label. Colors follow lipgloss's detected profile,
// so piping or NO_COLOR gives plain text.
func Text(r facts.Report) string {
	lines := textLines(r, true)
	box := lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Padding(0, 1)
	return box.Render(strings.Join(lines, "\n")) + "\n"
}

// Plain draws the label without a box or colors, for logs.
func Plain(r facts.Report) string {
	return strings.Join(textLines(r, false), "\n") + "\n"
}

func textLines(r facts.Report, styled bool) []string {
	st := func(s lipgloss.Style, v string) string {
		if !styled {
			return v
		}
		return s.Render(v)
	}
	var out []string
	for _, row := range Rows(r) {
		switch row.Kind {
		case Title:
			out = append(out, st(boldStyle, row.Label))
		case Sub, Note:
			label := fit(row.Label, Width)
			switch {
			case row.Bad:
				label = st(badStyle, label)
			case row.Bold:
				label = st(boldStyle, label)
			}
			out = append(out, label)
		case Thick:
			out = append(out, strings.Repeat("━", Width))
		case Thin:
			out = append(out, strings.Repeat("─", Width))
		case Header:
			out = append(out, st(boldStyle, spread(row.Label, row.DV)))
		case GradeRow:
			g := gradeStyle[r.Grade]
			out = append(out, st(g, row.Label)+pad(Width-len(row.Label)-len(row.Amount))+row.Amount)
		case Fact:
			out = append(out, factLine(row, st))
		}
	}
	return out
}

// factLine lays out: label ........ amount   dv
func factLine(row Row, st func(lipgloss.Style, string) string) string {
	const dvCol = 5
	label := row.Label
	if row.Indent {
		label = "  " + label
	}
	right := row.Amount + pad(dvCol+1-len(row.DV)) + row.DV
	space := Width - lipgloss.Width(label) - lipgloss.Width(right)
	if space < 1 {
		label = fit(label, Width-lipgloss.Width(right)-1)
		space = 1
	}
	amt := row.Amount
	if row.Bad {
		amt = st(badStyle, amt)
	}
	rightStyled := strings.Replace(right, row.Amount, amt, 1)
	if row.Bold && !row.Indent {
		label = st(boldStyle, label)
	}
	return label + pad(space) + rightStyled
}

func spread(left, right string) string {
	return left + pad(Width-len(left)-len(right)) + right
}

func pad(n int) string {
	if n < 0 {
		n = 0
	}
	return strings.Repeat(" ", n)
}

// fit truncates s to w columns with an ellipsis.
func fit(s string, w int) string {
	if lipgloss.Width(s) <= w {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && lipgloss.Width(string(r))+1 > w {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}

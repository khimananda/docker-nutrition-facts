// Package render draws a facts.Report as text, markdown, JSON or SVG.
package render

import (
	"fmt"
	"strings"

	"github.com/khimananda/docker-nutrition-facts/internal/facts"
)

// Kind is the type of a label row.
type Kind int

const (
	Title Kind = iota
	Sub
	Thick
	Thin
	Header
	Fact
	Note
	GradeRow
)

// Row is one line of the label. Every renderer draws the same rows.
type Row struct {
	Kind   Kind
	Label  string
	Amount string
	DV     string
	Indent bool
	Bold   bool
	Bad    bool // highlight: something worth fixing
}

// Rows builds the label.
func Rows(r facts.Report) []Row {
	sizeLabel := "Calories (compressed)"
	if !r.Compressed {
		sizeLabel = "Calories (uncompressed)"
	}
	hc, hcDV := "none", "0%"
	if r.Healthcheck {
		hc, hcDV = "yes", "100%"
	}
	root := "no"
	if r.Root {
		root = "YES"
	}
	base := ShortName(r.BaseImage)
	if base == "" {
		base = "unknown"
	}
	contains := "no known allergens"
	if len(r.Allergens) > 0 {
		contains = strings.Join(r.Allergens, ", ")
	}
	rows := []Row{
		{Kind: Title, Label: "Nutrition Facts"},
		{Kind: Sub, Label: fmt.Sprintf("%s  (%s)", ShortName(r.Ref), r.Platform)},
		{Kind: Sub, Label: "Serving size 1 container"},
		{Kind: Thick},
		{Kind: Header, Label: "Amount per serving", DV: "% Daily Value"},
		{Kind: Thin},
		{Kind: Fact, Label: sizeLabel, Amount: facts.HumanSize(r.SizeBytes), DV: pctStr(r.SizeDV), Bold: true, Bad: r.SizeDV > 100},
		{Kind: Fact, Label: "Total Layers", Amount: fmt.Sprint(r.Layers), DV: pctStr(r.LayersDV), Bold: true, Bad: r.LayersDV > 100},
		{Kind: Fact, Label: "Metadata-only steps", Amount: fmt.Sprint(r.EmptySteps), Indent: true},
		bad("Saturated Fat (pkg cache)", r.PackageCache, "layer"),
		bad("Trans Fat (curl | sh)", r.PipeToShell, "step"),
		bad("Sodium (secret-ish ENV)", len(r.SecretEnv), "var"),
		bad("Sugar (unpinned installs)", r.Unpinned, "cmd"),
		{Kind: Fact, Label: "Protein (HEALTHCHECK)", Amount: hc, DV: hcDV, Bold: true, Bad: !r.Healthcheck},
		{Kind: Thick},
		{Kind: Fact, Label: "Runs as root", Amount: root, Bad: r.Root},
		{Kind: Fact, Label: "Base image", Amount: base},
		{Kind: Fact, Label: "Best before", Amount: age(r)},
		{Kind: Thick},
		{Kind: Note, Label: "CONTAINS: " + contains, Bold: true, Bad: len(r.Allergens) > 0},
		{Kind: GradeRow, Label: fmt.Sprintf("Grade: %s", r.Grade), Amount: fmt.Sprintf("%d/100", r.Score)},
		{Kind: Thin},
		{Kind: Note, Label: "* Daily values based on a 100 MB diet."},
	}
	if r.Footnote != "" {
		rows = append(rows, Row{Kind: Note, Label: "* " + r.Footnote})
	}
	return rows
}

func bad(label string, n int, unit string) Row {
	u := unit
	if n != 1 {
		u += "s"
	}
	return Row{Kind: Fact, Label: label, Amount: fmt.Sprintf("%d %s", n, u), DV: pctStr(facts.BadDV(n)), Bold: true, Bad: n > 0}
}

func age(r facts.Report) string {
	switch {
	case r.Created.IsZero():
		return "unknown"
	case r.Created.Year() < 1990:
		return "timeless (epoch build)"
	case r.AgeDays == 0:
		return "built today"
	case r.AgeDays == 1:
		return "built 1 day ago"
	}
	return fmt.Sprintf("built %d days ago", r.AgeDays)
}

func pctStr(p int) string { return fmt.Sprintf("%d%%", p) }

// ShortName drops the implied Docker Hub prefixes: docker.io/library/nginx -> nginx.
func ShortName(ref string) string {
	for _, p := range []string{"docker.io/library/", "index.docker.io/library/", "docker.io/", "index.docker.io/"} {
		if strings.HasPrefix(ref, p) {
			return strings.TrimPrefix(ref, p)
		}
	}
	return ref
}

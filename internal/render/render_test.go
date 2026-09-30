package render

import (
	"encoding/json"
	"encoding/xml"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/khimananda/docker-nutrition-facts/internal/facts"
)

var update = flag.Bool("update", false, "rewrite golden files")

var now = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

func fixtures() map[string]facts.Report {
	good := facts.Analyze(facts.Input{
		Ref: "gcr.io/distroless/static-debian12:nonroot", Tag: "nonroot", Platform: "linux/amd64",
		Size: 1_990_000, Compressed: true, Layers: 3, User: "65532:65532", Healthcheck: true,
		Created: time.Unix(1, 0).UTC(), BaseImage: "scratch",
	}, now)
	good.Footnote = "Keep out of reach of production."

	bad := facts.Analyze(facts.Input{
		Ref: "docker.io/library/ubuntu:latest", Tag: "latest", Platform: "linux/amd64",
		Size: 1_240_000_000, Compressed: true, Layers: 37, EnvKeys: []string{"PATH", "DB_PASSWORD"},
		Created: now.AddDate(0, 0, -212), BaseImage: "docker.io/library/ubuntu:22.04",
		History: []facts.History{
			{CreatedBy: "RUN /bin/sh -c apt-get update && apt-get install -y curl sudo # buildkit"},
			{CreatedBy: "RUN /bin/sh -c curl -fsSL https://get.example.sh | bash # buildkit"},
			{CreatedBy: "ENV DB_PASSWORD=hunter2", Empty: true},
		},
	}, now)
	bad.Footnote = "May contain traces of node_modules."
	return map[string]facts.Report{"good": good, "bad": bad}
}

func golden(t *testing.T, name, got string) {
	t.Helper()
	p := filepath.Join("testdata", name)
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("missing golden file, run go test ./... -update: %v", err)
	}
	if got != string(want) {
		t.Errorf("%s differs from golden file.\n--- got\n%s\n--- want\n%s", name, got, want)
	}
}

func TestGolden(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)
	for name, r := range fixtures() {
		t.Run(name, func(t *testing.T) {
			golden(t, name+".txt", Text(r))
			golden(t, name+".plain.txt", Plain(r))
			golden(t, name+".md", Markdown(r))
			golden(t, name+".svg", SVG(r))
			js, err := JSON(r)
			if err != nil {
				t.Fatal(err)
			}
			golden(t, name+".json", js)
		})
	}
}

func TestTextLinesFitTheBox(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)
	for name, r := range fixtures() {
		for i, l := range strings.Split(strings.TrimRight(Text(r), "\n"), "\n") {
			if w := lipgloss.Width(l); w != Width+4 {
				t.Errorf("%s line %d is %d wide, want %d: %q", name, i, w, Width+4, l)
			}
		}
	}
}

func TestNoColorHasNoEscapes(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)
	if strings.Contains(Text(fixtures()["bad"]), "\x1b[") {
		t.Error("ASCII profile output contains escape codes")
	}
}

func TestNeverPrintsEnvValues(t *testing.T) {
	r := fixtures()["bad"]
	js, _ := JSON(r)
	for _, out := range []string{Text(r), Plain(r), Markdown(r), SVG(r), js} {
		if strings.Contains(out, "hunter2") {
			t.Fatal("an env value leaked into output")
		}
	}
}

func TestSVGIsWellFormed(t *testing.T) {
	for name, r := range fixtures() {
		d := xml.NewDecoder(strings.NewReader(SVG(r)))
		for {
			_, err := d.Token()
			if err != nil {
				if err.Error() != "EOF" {
					t.Errorf("%s: %v", name, err)
				}
				break
			}
		}
	}
}

func TestJSONRoundTrip(t *testing.T) {
	js, _ := JSON(fixtures()["bad"])
	var back facts.Report
	if err := json.Unmarshal([]byte(js), &back); err != nil || back.Grade != "F" {
		t.Errorf("round trip failed: %v grade %q", err, back.Grade)
	}
}

func TestShortName(t *testing.T) {
	for in, want := range map[string]string{"docker.io/library/nginx:1": "nginx:1", "docker.io/grafana/grafana": "grafana/grafana", "ghcr.io/a/b": "ghcr.io/a/b"} {
		if got := ShortName(in); got != want {
			t.Errorf("%s: got %s", in, got)
		}
	}
}

func TestLongValuesKeepTheirLabel(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)
	r := fixtures()["good"]
	r.BaseImage = "gcr.io/distroless/static-debian12:nonroot@sha256:0123456789abcdef"
	out := Text(r)
	if !strings.Contains(out, "Base image") || !strings.Contains(out, "gcr.io/distroless/") {
		t.Errorf("label or value missing:\n%s", out)
	}
	for i, l := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if w := lipgloss.Width(l); w != Width+4 {
			t.Errorf("line %d is %d wide: %q", i, w, l)
		}
	}
}

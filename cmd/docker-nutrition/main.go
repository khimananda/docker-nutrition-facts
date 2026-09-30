// Command docker-nutrition prints a Nutrition Facts label for a container image.
// It also works as a Docker CLI plugin: docker nutrition <image>.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/spf13/cobra"

	"github.com/khimananda/docker-nutrition-facts/internal/facts"
	"github.com/khimananda/docker-nutrition-facts/internal/inspect"
	"github.com/khimananda/docker-nutrition-facts/internal/phrases"
	"github.com/khimananda/docker-nutrition-facts/internal/render"
)

// Set by GoReleaser. go install builds fall back to the module version.
var version = "dev"

func init() {
	if version != "dev" {
		return
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		version = strings.TrimPrefix(bi.Main.Version, "v")
	}
}

const repoURL = "https://github.com/khimananda/docker-nutrition-facts"

// errGrade marks a --fail-below failure so main can exit 2 without noise.
var errGrade = errors.New("grade below threshold")

func main() {
	args := os.Args[1:]
	// Docker CLI plugin protocol: answer the metadata probe, and strip the
	// plugin name Docker passes as the first argument.
	if len(args) > 0 && args[0] == "docker-cli-plugin-metadata" {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]string{
			"SchemaVersion":    "0.1.0",
			"Vendor":           "khimananda",
			"Version":          version,
			"ShortDescription": "Print a Nutrition Facts label for an image",
			"URL":              repoURL,
		})
		return
	}
	if len(args) > 0 && args[0] == "nutrition" {
		args = args[1:]
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	cmd := newRoot(os.Stdout)
	cmd.SetArgs(args)
	if err := cmd.ExecuteContext(ctx); err != nil {
		if errors.Is(err, errGrade) {
			os.Exit(2)
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

type flags struct {
	platform  string
	format    string
	local     bool
	insecure  bool
	noColor   bool
	failBelow string
	output    string
}

func newRoot(stdout io.Writer) *cobra.Command {
	var f flags
	cmd := &cobra.Command{
		Use:   "docker-nutrition IMAGE",
		Short: "Print an FDA-style Nutrition Facts label for a container image",
		Long: `Print an FDA-style Nutrition Facts label for a container image.

Reads the manifest and config only, so it is fast even on huge images.
Env var values are never read or printed. Only their names are checked.`,
		Example: `  docker-nutrition nginx:latest
  docker nutrition ghcr.io/org/app:1.2.3 --platform linux/arm64
  docker-nutrition myapp:dev --local
  docker-nutrition node:22 --format svg -o label.svg
  docker-nutrition "$IMAGE" --format markdown --fail-below C`,
		Version:       version,
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd.Context(), stdout, args[0], f)
		},
	}
	fl := cmd.Flags()
	fl.StringVar(&f.platform, "platform", "linux/amd64", "platform to inspect for multi-arch images (os/arch[/variant])")
	fl.StringVarP(&f.format, "format", "f", "text", "output format: text, plain, json, markdown, svg")
	fl.BoolVar(&f.local, "local", false, "read the image from the local Docker daemon (uses docker save)")
	fl.BoolVar(&f.insecure, "insecure", false, "allow plain HTTP registries")
	fl.BoolVar(&f.noColor, "no-color", false, "disable colors (NO_COLOR is also honoured)")
	fl.StringVar(&f.failBelow, "fail-below", "", "exit with code 2 when the grade is worse than this (A-F)")
	fl.StringVarP(&f.output, "output", "o", "", "write to a file instead of stdout")
	return cmd
}

func run(ctx context.Context, stdout io.Writer, ref string, f flags) error {
	if f.failBelow != "" && facts.GradeRank(f.failBelow) < 0 {
		return fmt.Errorf("--fail-below must be one of A, B, C, D, F")
	}
	if f.noColor || os.Getenv("NO_COLOR") != "" || f.output != "" {
		lipgloss.SetColorProfile(termenv.Ascii)
	}

	in, err := inspect.Fetch(ctx, ref, inspect.Options{Platform: f.platform, Local: f.local, Insecure: f.insecure})
	if err != nil {
		return err
	}
	rep := facts.Analyze(in, time.Now())
	rep.Footnote = phrases.Footnote(rep.Digest)

	var out string
	switch strings.ToLower(f.format) {
	case "text":
		out = render.Text(rep)
	case "plain":
		out = render.Plain(rep)
	case "markdown", "md":
		out = render.Markdown(rep)
	case "svg":
		out = render.SVG(rep)
	case "json":
		if out, err = render.JSON(rep); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown --format %q: use text, plain, json, markdown or svg", f.format)
	}

	w := stdout
	if f.output != "" {
		file, err := os.Create(f.output)
		if err != nil {
			return err
		}
		defer file.Close()
		w = file
	}
	if _, err := io.WriteString(w, out); err != nil {
		return err
	}
	if f.failBelow != "" && facts.GradeRank(rep.Grade) < facts.GradeRank(f.failBelow) {
		fmt.Fprintf(os.Stderr, "grade %s is below the required %s\n", rep.Grade, strings.ToUpper(f.failBelow))
		return errGrade
	}
	return nil
}

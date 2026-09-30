package facts

import (
	"fmt"
	"strings"
	"time"
)

// Recommended daily amounts. The joke has a real scale.
const (
	DailySizeBytes = 100 * 1000 * 1000 // a 100 MB diet
	DailyLayers    = 15
)

// Report is the finished label, ready for any renderer.
type Report struct {
	Ref          string    `json:"ref"`
	Platform     string    `json:"platform"`
	Digest       string    `json:"digest,omitempty"`
	SizeBytes    int64     `json:"size_bytes"`
	Compressed   bool      `json:"size_is_compressed"`
	SizeDV       int       `json:"size_daily_value_pct"`
	Layers       int       `json:"layers"`
	EmptySteps   int       `json:"metadata_only_steps"`
	LayersDV     int       `json:"layers_daily_value_pct"`
	PackageCache int       `json:"package_cache_layers"`
	PipeToShell  int       `json:"curl_pipe_sh_steps"`
	SecretEnv    []string  `json:"secret_looking_env"`
	Unpinned     int       `json:"unpinned_install_commands"`
	Healthcheck  bool      `json:"healthcheck"`
	Root         bool      `json:"runs_as_root"`
	BaseImage    string    `json:"base_image,omitempty"`
	Created      time.Time `json:"created"`
	AgeDays      int       `json:"age_days"`
	ExposedPorts int       `json:"exposed_ports"`
	Allergens    []string  `json:"allergens"`
	Score        int       `json:"score"`
	Grade        string    `json:"grade"`
	Deductions   []string  `json:"deductions"`
	Footnote     string    `json:"footnote"`
}

// Analyze builds a Report from Input. now is injected so tests are stable.
func Analyze(in Input, now time.Time) Report {
	r := Report{
		Ref:          in.Ref,
		Platform:     in.Platform,
		Digest:       in.Digest,
		SizeBytes:    in.Size,
		Compressed:   in.Compressed,
		SizeDV:       pct(in.Size, DailySizeBytes),
		Layers:       in.Layers,
		LayersDV:     pct(int64(in.Layers), DailyLayers),
		PackageCache: PackageCacheLayers(in.History),
		PipeToShell:  PipeToShell(in.History),
		SecretEnv:    SecretLookingEnv(in.EnvKeys),
		Unpinned:     UnpinnedInstalls(in.History),
		Healthcheck:  in.Healthcheck,
		Root:         RunsAsRoot(in.User),
		BaseImage:    in.BaseImage,
		Created:      in.Created,
		AgeDays:      -1,
		ExposedPorts: in.ExposedPorts,
	}
	for _, h := range in.History {
		if h.Empty {
			r.EmptySteps++
		}
	}
	if r.SecretEnv == nil {
		r.SecretEnv = []string{}
	}
	if !in.Created.IsZero() && in.Created.Year() >= 1990 {
		r.AgeDays = int(now.Sub(in.Created).Hours() / 24)
	}

	if in.Tag == "latest" {
		r.Allergens = append(r.Allergens, ":latest")
	}
	if r.Root {
		r.Allergens = append(r.Allergens, "root")
	}
	if r.PipeToShell > 0 {
		r.Allergens = append(r.Allergens, "curl | sh")
	}
	if Installs(in.History, "sudo") {
		r.Allergens = append(r.Allergens, "sudo")
	}
	if Installs(in.History, "openssh-server") {
		r.Allergens = append(r.Allergens, "sshd")
	}
	if r.Allergens == nil {
		r.Allergens = []string{}
	}

	r.Score, r.Deductions = score(r, in.Tag)
	r.Grade = Grade(r.Score)
	return r
}

func score(r Report, tag string) (int, []string) {
	s := 100
	var why []string
	take := func(n int, reason string) {
		if n > 0 {
			s -= n
			why = append(why, fmt.Sprintf("-%d %s", n, reason))
		}
	}
	if r.Root {
		take(15, "runs as root")
	}
	if tag == "latest" {
		take(10, "uses the :latest tag")
	}
	take(min(15*r.PipeToShell, 30), "pipes downloads into a shell")
	take(min(15*len(r.SecretEnv), 30), "secret-looking env vars")
	take(min(5*r.PackageCache, 15), "package cache left in layers")
	take(min(5*r.Unpinned, 10), "unpinned package installs")
	if !r.Healthcheck {
		take(5, "no HEALTHCHECK")
	}
	switch {
	case r.SizeBytes > 1000*1000*1000:
		take(20, "over 1 GB")
	case r.SizeBytes > 500*1000*1000:
		take(10, "over 500 MB")
	}
	if r.Layers > 30 {
		take(5, "more than 30 layers")
	}
	if r.AgeDays > 180 {
		take(10, "built more than 180 days ago")
	}
	if s < 0 {
		s = 0
	}
	if why == nil {
		why = []string{}
	}
	return s, why
}

// Grade maps a 0 to 100 score to a letter.
func Grade(score int) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 80:
		return "B"
	case score >= 70:
		return "C"
	case score >= 60:
		return "D"
	}
	return "F"
}

// GradeRank orders grades so A > B > ... > F. Unknown grades rank lowest.
func GradeRank(g string) int {
	return strings.Index("FDCBA", strings.ToUpper(g))
}

func pct(v, daily int64) int {
	if daily <= 0 {
		return 0
	}
	return int((v*100 + daily/2) / daily)
}

// BadDV turns a count of something with a zero recommended intake into a
// daily value: each occurrence is worth 50%.
func BadDV(n int) int { return n * 50 }

// HumanSize formats bytes with decimal units, like registries do.
func HumanSize(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "kMGTPE"[exp])
}

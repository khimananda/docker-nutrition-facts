package facts

import (
	"strings"
	"testing"
	"time"
)

var now = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

func TestAnalyzeCleanImage(t *testing.T) {
	r := Analyze(Input{
		Ref: "gcr.io/distroless/static:nonroot", Tag: "nonroot", Size: 2_000_000, Compressed: true,
		Layers: 3, User: "65532:65532", Healthcheck: true, Created: now.Add(-48 * time.Hour),
	}, now)
	if r.Grade != "A" || r.Score != 100 {
		t.Errorf("grade %s score %d, deductions %v", r.Grade, r.Score, r.Deductions)
	}
	if len(r.Allergens) != 0 || r.AgeDays != 2 {
		t.Errorf("allergens %v age %d", r.Allergens, r.AgeDays)
	}
}

func TestAnalyzeScaryImage(t *testing.T) {
	r := Analyze(Input{
		Ref: "ubuntu:latest", Tag: "latest", Size: 1_200_000_000, Layers: 40,
		EnvKeys: []string{"DB_PASSWORD", "PATH"}, Created: now.AddDate(0, 0, -400),
		History: run("apt-get update && apt-get install -y curl sudo", "curl https://x.example | bash"),
	}, now)
	if r.Grade != "F" {
		t.Errorf("grade %s score %d", r.Grade, r.Score)
	}
	got := strings.Join(r.Allergens, ",")
	if got != ":latest,root,curl | sh,sudo" {
		t.Errorf("allergens %q", got)
	}
	if r.SizeDV != 1200 || r.LayersDV != 267 {
		t.Errorf("daily values size %d layers %d", r.SizeDV, r.LayersDV)
	}
}

func TestEpochBuildHasUnknownAge(t *testing.T) {
	r := Analyze(Input{Created: time.Unix(0, 0).UTC()}, now)
	if r.AgeDays != -1 {
		t.Errorf("epoch builds should not be aged, got %d", r.AgeDays)
	}
	if r.AgeDays > 180 {
		t.Error("epoch build must not be penalised for age")
	}
}

func TestGrade(t *testing.T) {
	for s, g := range map[int]string{100: "A", 90: "A", 89: "B", 80: "B", 70: "C", 60: "D", 59: "F", 0: "F"} {
		if Grade(s) != g {
			t.Errorf("Grade(%d) = %s, want %s", s, Grade(s), g)
		}
	}
	if !(GradeRank("A") > GradeRank("C") && GradeRank("c") == GradeRank("C") && GradeRank("Z") == -1) {
		t.Error("GradeRank ordering")
	}
}

func TestHumanSize(t *testing.T) {
	for b, want := range map[int64]string{512: "512 B", 1500: "1.5 kB", 67_200_000: "67.2 MB", 2_000_000_000: "2.0 GB"} {
		if got := HumanSize(b); got != want {
			t.Errorf("HumanSize(%d) = %s, want %s", b, got, want)
		}
	}
}

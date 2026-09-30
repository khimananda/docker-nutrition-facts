package main

import (
	"bytes"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

func pushRandom(t *testing.T) string {
	t.Helper()
	s := httptest.NewServer(registry.New())
	t.Cleanup(s.Close)
	ref := strings.TrimPrefix(s.URL, "http://") + "/demo/app:latest"
	tag, _ := name.NewTag(ref)
	img, _ := random.Image(512, 2)
	if err := remote.Write(tag, img); err != nil {
		t.Fatal(err)
	}
	return ref
}

func execute(t *testing.T, args ...string) (string, error) {
	var out bytes.Buffer
	cmd := newRoot(&out)
	cmd.SetArgs(args)
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	return out.String(), err
}

func TestFormats(t *testing.T) {
	ref := pushRandom(t)
	for format, want := range map[string]string{"text": "Nutrition Facts", "plain": "Serving size", "json": `"grade"`, "markdown": "<!-- docker-nutrition-facts -->", "svg": "<svg"} {
		out, err := execute(t, ref, "--insecure", "-f", format)
		if err != nil || !strings.Contains(out, want) {
			t.Errorf("%s: err %v, output missing %q", format, err, want)
		}
	}
	if _, err := execute(t, ref, "--insecure", "-f", "yaml"); err == nil {
		t.Error("expected unknown format error")
	}
}

func TestFailBelow(t *testing.T) {
	ref := pushRandom(t) // root, :latest, no healthcheck: grade B at best
	if _, err := execute(t, ref, "--insecure", "-f", "json", "--fail-below", "F"); err != nil {
		t.Errorf("F threshold should always pass: %v", err)
	}
	if _, err := execute(t, ref, "--insecure", "-f", "json", "--fail-below", "A"); !errors.Is(err, errGrade) {
		t.Errorf("expected errGrade, got %v", err)
	}
	if _, err := execute(t, ref, "--fail-below", "Q"); err == nil {
		t.Error("expected validation error for bad grade")
	}
}

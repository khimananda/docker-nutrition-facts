package inspect

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

func testImage(t *testing.T, arch string) v1.Image {
	t.Helper()
	img, err := random.Image(1024, 3)
	if err != nil {
		t.Fatal(err)
	}
	cf, err := img.ConfigFile()
	if err != nil {
		t.Fatal(err)
	}
	cf = cf.DeepCopy()
	cf.OS, cf.Architecture = "linux", arch
	cf.Config.User = "app"
	cf.Config.Env = []string{"PATH=/usr/bin", "API_TOKEN=supersecret"}
	cf.Config.Healthcheck = &v1.HealthConfig{Test: []string{"CMD", "true"}}
	cf.History = []v1.History{
		{CreatedBy: "RUN /bin/sh -c apt-get install -y curl # buildkit"},
		{CreatedBy: "ENV API_TOKEN=supersecret", EmptyLayer: true},
	}
	img, err = mutate.ConfigFile(img, cf)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func TestFetchSingleImage(t *testing.T) {
	s := httptest.NewServer(registry.New())
	defer s.Close()
	ref := strings.TrimPrefix(s.URL, "http://") + "/team/app:latest"
	tag, _ := name.NewTag(ref)
	img := testImage(t, "amd64")
	if err := remote.Write(tag, img); err != nil {
		t.Fatal(err)
	}

	in, err := Fetch(context.Background(), ref, Options{Platform: "linux/amd64", Insecure: true})
	if err != nil {
		t.Fatal(err)
	}
	if in.Layers != 3 || in.Tag != "latest" || in.User != "app" || !in.Healthcheck || !in.Compressed {
		t.Errorf("unexpected input: %+v", in)
	}
	if len(in.EnvKeys) != 2 || in.EnvKeys[1] != "API_TOKEN" {
		t.Errorf("env keys %v", in.EnvKeys)
	}
	m, _ := img.Manifest()
	var want int64
	for _, l := range m.Layers {
		want += l.Size
	}
	if in.Size != want {
		t.Errorf("size %d, want %d", in.Size, want)
	}
	if len(in.History) != 2 || !in.History[1].Empty {
		t.Errorf("history %+v", in.History)
	}
}

func TestFetchIndexPicksPlatformAndAnnotation(t *testing.T) {
	s := httptest.NewServer(registry.New())
	defer s.Close()
	ref := strings.TrimPrefix(s.URL, "http://") + "/team/multi:1.0"
	tag, _ := name.NewTag(ref)

	idx := mutate.AppendManifests(empty.Index,
		mutate.IndexAddendum{Add: testImage(t, "amd64"), Descriptor: v1.Descriptor{
			Platform: &v1.Platform{OS: "linux", Architecture: "amd64"}}},
		mutate.IndexAddendum{Add: testImage(t, "arm64"), Descriptor: v1.Descriptor{
			Platform:    &v1.Platform{OS: "linux", Architecture: "arm64", Variant: "v8"},
			Annotations: map[string]string{BaseNameKey: "docker.io/library/debian:12"}}},
	)
	if err := remote.WriteIndex(tag, idx); err != nil {
		t.Fatal(err)
	}

	in, err := Fetch(context.Background(), ref, Options{Platform: "linux/arm64", Insecure: true})
	if err != nil {
		t.Fatal(err)
	}
	if in.Platform != "linux/arm64" || in.BaseImage != "docker.io/library/debian:12" || in.Tag != "1.0" {
		t.Errorf("got platform %q base %q tag %q", in.Platform, in.BaseImage, in.Tag)
	}

	_, err = Fetch(context.Background(), ref, Options{Platform: "linux/s390x", Insecure: true})
	if err == nil || !strings.Contains(err.Error(), "linux/amd64, linux/arm64/v8") {
		t.Errorf("expected a helpful platform error, got %v", err)
	}
}

func TestFetchNotFound(t *testing.T) {
	s := httptest.NewServer(registry.New())
	defer s.Close()
	ref := strings.TrimPrefix(s.URL, "http://") + "/nope/nothing:1"
	_, err := Fetch(context.Background(), ref, Options{Platform: "linux/amd64", Insecure: true})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not found, got %v", err)
	}
}

func TestFetchBadInput(t *testing.T) {
	if _, err := Fetch(context.Background(), "UPPER CASE", Options{Platform: "linux/amd64"}); err == nil {
		t.Error("expected invalid reference error")
	}
}

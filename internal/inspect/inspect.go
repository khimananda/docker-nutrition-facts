// Package inspect fetches image metadata. It reads the manifest and config
// only and never downloads layer blobs from a registry.
package inspect

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"

	"github.com/khimananda/docker-nutrition-facts/internal/facts"
)

// BaseNameKey is the OCI annotation for the base image name.
const BaseNameKey = "org.opencontainers.image.base.name"

// Options control how an image is fetched.
type Options struct {
	Platform string // os/arch[/variant]
	Local    bool   // read from the local Docker daemon via docker save
	Insecure bool   // allow plain HTTP registries
	Remote   []remote.Option
}

// Fetch returns facts.Input for ref.
func Fetch(ctx context.Context, ref string, o Options) (facts.Input, error) {
	opts := []name.Option{}
	if o.Insecure {
		opts = append(opts, name.Insecure)
	}
	r, err := name.ParseReference(ref, opts...)
	if err != nil {
		return facts.Input{}, fmt.Errorf("%q is not a valid image reference: %w", ref, err)
	}
	plat, err := v1.ParsePlatform(o.Platform)
	if err != nil {
		return facts.Input{}, fmt.Errorf("invalid --platform %q: %w", o.Platform, err)
	}
	if o.Local {
		return fetchLocal(ctx, ref, r, *plat)
	}
	return fetchRemote(ctx, ref, r, *plat, o)
}

func fetchRemote(ctx context.Context, raw string, r name.Reference, plat v1.Platform, o Options) (facts.Input, error) {
	ropts := append([]remote.Option{remote.WithContext(ctx), remote.WithAuthFromKeychain(authn.DefaultKeychain)}, o.Remote...)
	desc, err := remote.Get(r, ropts...)
	if err != nil {
		return facts.Input{}, explain(raw, err)
	}
	annotations := map[string]string{}
	var img v1.Image
	if desc.MediaType.IsIndex() {
		idx, err := desc.ImageIndex()
		if err != nil {
			return facts.Input{}, err
		}
		im, err := idx.IndexManifest()
		if err != nil {
			return facts.Input{}, err
		}
		var chosen *v1.Descriptor
		var available []string
		for i := range im.Manifests {
			m := im.Manifests[i]
			if m.Platform == nil || m.Platform.OS == "unknown" {
				continue // attestation manifests
			}
			available = append(available, platformString(*m.Platform))
			if matches(*m.Platform, plat) && chosen == nil {
				chosen = &m
			}
		}
		if chosen == nil {
			return facts.Input{}, fmt.Errorf("%s has no %s variant. Available: %s. Use --platform",
				raw, platformString(plat), strings.Join(available, ", "))
		}
		for k, v := range chosen.Annotations {
			annotations[k] = v
		}
		if img, err = idx.Image(chosen.Digest); err != nil {
			return facts.Input{}, err
		}
	} else {
		if img, err = desc.Image(); err != nil {
			return facts.Input{}, err
		}
	}
	if m, err := img.Manifest(); err == nil {
		for k, v := range m.Annotations {
			annotations[k] = v
		}
	}
	in, err := fromImage(img, true)
	if err != nil {
		return facts.Input{}, err
	}
	in.Ref = raw
	in.Tag = tagOf(r)
	if in.BaseImage == "" {
		in.BaseImage = annotations[BaseNameKey]
	}
	return in, nil
}

func fetchLocal(ctx context.Context, raw string, r name.Reference, plat v1.Platform) (facts.Input, error) {
	dir, err := os.MkdirTemp("", "docker-nutrition-*")
	if err != nil {
		return facts.Input{}, err
	}
	defer os.RemoveAll(dir)
	tar := filepath.Join(dir, "image.tar")
	cmd := exec.CommandContext(ctx, "docker", "save", "-o", tar, raw)
	if out, err := cmd.CombinedOutput(); err != nil {
		return facts.Input{}, fmt.Errorf("docker save %s failed: %s", raw, strings.TrimSpace(string(out)))
	}
	var tag *name.Tag
	if t, ok := r.(name.Tag); ok {
		tag = &t
	}
	img, err := tarball.ImageFromPath(tar, tag)
	if err != nil {
		// Some tarballs list the image under a normalised name only.
		if img, err = tarball.ImageFromPath(tar, nil); err != nil {
			return facts.Input{}, fmt.Errorf("reading docker save output: %w", err)
		}
	}
	in, err := fromImage(img, false)
	if err != nil {
		return facts.Input{}, err
	}
	in.Ref = raw
	in.Tag = tagOf(r)
	if in.Platform == "" {
		in.Platform = platformString(plat)
	}
	return in, nil
}

func fromImage(img v1.Image, compressed bool) (facts.Input, error) {
	cf, err := img.ConfigFile()
	if err != nil {
		return facts.Input{}, fmt.Errorf("reading image config: %w", err)
	}
	layers, err := img.Layers()
	if err != nil {
		return facts.Input{}, fmt.Errorf("reading layers: %w", err)
	}
	var size int64
	if compressed {
		m, err := img.Manifest()
		if err != nil {
			return facts.Input{}, err
		}
		for _, l := range m.Layers {
			size += l.Size
		}
	} else {
		for _, l := range layers {
			s, err := l.Size()
			if err != nil {
				return facts.Input{}, err
			}
			size += s
		}
	}
	d, _ := img.Digest()
	in := facts.Input{
		Digest:     d.String(),
		OS:         cf.OS,
		Size:       size,
		Compressed: compressed,
		Layers:     len(layers),
		Created:    cf.Created.Time,
	}
	if cf.OS != "" {
		in.Platform = platformString(v1.Platform{OS: cf.OS, Architecture: cf.Architecture, Variant: cf.Variant})
	}
	for _, h := range cf.History {
		in.History = append(in.History, facts.History{CreatedBy: h.CreatedBy, Empty: h.EmptyLayer})
	}
	c := cf.Config
	in.User = c.User
	for _, e := range c.Env {
		k, _, _ := strings.Cut(e, "=")
		in.EnvKeys = append(in.EnvKeys, k)
	}
	if hc := c.Healthcheck; hc != nil && len(hc.Test) > 0 && hc.Test[0] != "NONE" {
		in.Healthcheck = true
	}
	in.ExposedPorts = len(c.ExposedPorts)
	in.BaseImage = c.Labels[BaseNameKey]
	return in, nil
}

func matches(have, want v1.Platform) bool {
	if have.OS != want.OS || have.Architecture != want.Architecture {
		return false
	}
	return want.Variant == "" || have.Variant == want.Variant
}

func platformString(p v1.Platform) string {
	s := p.OS + "/" + p.Architecture
	if p.Variant != "" {
		s += "/" + p.Variant
	}
	return s
}

func tagOf(r name.Reference) string {
	if t, ok := r.(name.Tag); ok {
		return t.TagStr()
	}
	return ""
}

func explain(ref string, err error) error {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "MANIFEST_UNKNOWN"), strings.Contains(msg, "NAME_UNKNOWN"):
		return fmt.Errorf("image %s not found in the registry", ref)
	case strings.Contains(msg, "UNAUTHORIZED"), strings.Contains(msg, "DENIED"):
		return fmt.Errorf("not allowed to read %s. Run docker login for that registry, or check the name: %w", ref, err)
	case strings.Contains(msg, "TOOMANYREQUESTS"):
		return fmt.Errorf("the registry is rate limiting you. Run docker login and try again: %w", err)
	}
	return fmt.Errorf("fetching %s: %w", ref, err)
}

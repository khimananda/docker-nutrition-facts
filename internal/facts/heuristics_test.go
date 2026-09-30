package facts

import (
	"reflect"
	"testing"
)

func run(cmds ...string) []History {
	var h []History
	for _, c := range cmds {
		h = append(h, History{CreatedBy: "RUN /bin/sh -c " + c + " # buildkit"})
	}
	return h
}

func TestPackageCacheLayers(t *testing.T) {
	cases := []struct {
		name string
		cmd  string
		want int
	}{
		{"apt no cleanup", "apt-get update && apt-get install -y curl", 1},
		{"apt with lists cleanup", "apt-get update && apt-get install -y curl && rm -rf /var/lib/apt/lists/*", 0},
		{"apt dist-clean", "apt-get update; apt-get install -y curl; apt-get dist-clean", 0},
		{"apk no cache flag", "apk add curl", 1},
		{"apk --no-cache", "apk add --no-cache curl", 0},
		{"dnf no clean", "dnf install -y git", 1},
		{"dnf clean all", "dnf install -y git && dnf clean all", 0},
		{"unrelated", "echo hello", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := PackageCacheLayers(run(c.cmd)); got != c.want {
				t.Errorf("got %d, want %d", got, c.want)
			}
		})
	}
}

func TestPipeToShell(t *testing.T) {
	cases := map[string]int{
		"curl -fsSL https://get.docker.com | sh":                  1,
		"curl -sL https://deb.nodesource.com/setup_22.x | bash -": 1,
		"wget -qO- https://x.example/install.sh | sudo bash":      1,
		"curl https://x.example | sh -s -- --yes":                 1,
		"curl -o install.sh https://x.example && sh install.sh":   0,
		"curl https://x.example | tee out.txt":                    0,
		"echo curl is great | shasum":                             0,
	}
	for cmd, want := range cases {
		if got := PipeToShell(run(cmd)); got != want {
			t.Errorf("%q: got %d, want %d", cmd, got, want)
		}
	}
}

func TestSecretLookingEnv(t *testing.T) {
	got := SecretLookingEnv([]string{"PATH", "DB_PASSWORD", "GITHUB_TOKEN", "API_KEY", "PASSWORD_FILE", "NODE_VERSION", "AWS_SECRET_ACCESS_KEY", "TOKENIZERS_PARALLELISM"})
	want := []string{"DB_PASSWORD", "GITHUB_TOKEN", "API_KEY", "AWS_SECRET_ACCESS_KEY", "TOKENIZERS_PARALLELISM"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestUnpinnedInstalls(t *testing.T) {
	cases := []struct {
		cmd  string
		want int
	}{
		{"apt-get install -y curl", 1},
		{"apt-get install -y --no-install-recommends curl=8.5.0-2", 0},
		{"apt-get update && apt-get install -y a=1 b && apt-get install -y c=2", 1},
		{"apk add --no-cache curl=8.9.1-r0", 0},
		{"apk add --no-cache curl", 1},
		{"pip install -r requirements.txt", 0},
		{"pip install --no-cache-dir flask==3.0.0", 0},
		{"pip install flask", 1},
		{"python -m pip install flask", 1},
		{"pip install .", 0},
		{"npm install", 0},
		{"npm install -g pnpm@9.1.0", 0},
		{"npm i -g typescript", 1},
		{"npm install -g @angular/cli", 1},
		{"npm install -g @angular/cli@18.0.0", 0},
		{"DEBIAN_FRONTEND=noninteractive apt-get install -y git", 1},
		{"sudo apt-get install -y git", 1},
		{"echo apt-get install lies", 0},
	}
	for _, c := range cases {
		if got := UnpinnedInstalls(run(c.cmd)); got != c.want {
			t.Errorf("%q: got %d, want %d", c.cmd, got, c.want)
		}
	}
}

func TestRunCommandsSkipsMetadata(t *testing.T) {
	h := []History{
		{CreatedBy: "/bin/sh -c #(nop)  CMD [\"bash\"]", Empty: true},
		{CreatedBy: "ENV PATH=/usr/local/bin", Empty: true},
		{CreatedBy: "COPY app /app # buildkit"},
		{CreatedBy: "RUN |2 A=b C=d /bin/sh -c apt-get install -y curl # buildkit"},
		{CreatedBy: "/bin/sh -c apt-get install -y git"},
	}
	got := runCommands(h)
	if len(got) != 2 {
		t.Fatalf("got %d commands: %q", len(got), got)
	}
	if UnpinnedInstalls(h) != 2 {
		t.Errorf("expected both build-arg and classic builder steps to be parsed")
	}
}

func TestInstalls(t *testing.T) {
	h := run("apt-get update && apt-get install -y openssh-server sudo")
	if !Installs(h, "sudo") || !Installs(h, "openssh-server") {
		t.Error("expected sudo and openssh-server to be detected")
	}
	if Installs(run("apt-get install -y sudoku"), "sudo") {
		t.Error("sudoku is not sudo")
	}
	if Installs(run("echo sudo"), "sudo") {
		t.Error("echo is not an install")
	}
}

func TestRunsAsRoot(t *testing.T) {
	for u, want := range map[string]bool{"": true, "root": true, "0": true, "0:0": true, "root:root": true,
		"nginx": false, "1000": false, "65532:65532": false, "nonroot": false} {
		if got := RunsAsRoot(u); got != want {
			t.Errorf("%q: got %v, want %v", u, got, want)
		}
	}
}

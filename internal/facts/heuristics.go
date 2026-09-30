package facts

import (
	"regexp"
	"strings"
)

// runCommands returns the shell commands from build history, one per step,
// with the builder prefix removed.
func runCommands(h []History) []string {
	var out []string
	for _, s := range h {
		c := strings.TrimSpace(s.CreatedBy)
		if c == "" || strings.Contains(c, "#(nop)") {
			continue
		}
		c = strings.TrimPrefix(c, "RUN ")
		// BuildKit prefixes build args as "|2 A=b C=d /bin/sh -c ...".
		if i := strings.Index(c, "/bin/sh -c "); i >= 0 {
			c = c[i+len("/bin/sh -c "):]
		} else if strings.HasPrefix(c, "|") {
			continue
		}
		c = strings.TrimSpace(strings.TrimSuffix(c, "# buildkit"))
		if isMetadataStep(c) {
			continue
		}
		out = append(out, c)
	}
	return out
}

var metadataSteps = []string{"ENV ", "LABEL ", "CMD ", "ENTRYPOINT ", "EXPOSE ", "USER ", "WORKDIR ",
	"ARG ", "STOPSIGNAL ", "HEALTHCHECK ", "VOLUME ", "SHELL ", "ONBUILD ", "COPY ", "ADD "}

func isMetadataStep(c string) bool {
	for _, p := range metadataSteps {
		if strings.HasPrefix(c, p) {
			return true
		}
	}
	return false
}

// PackageCacheLayers counts steps that install OS packages and leave the cache behind.
func PackageCacheLayers(h []History) int {
	n := 0
	for _, c := range runCommands(h) {
		switch {
		case (strings.Contains(c, "apt-get install") || strings.Contains(c, "apt install")) &&
			!strings.Contains(c, "/var/lib/apt/lists") && !strings.Contains(c, "apt-get dist-clean"):
			n++
		case strings.Contains(c, "apk add") && !strings.Contains(c, "--no-cache") &&
			!strings.Contains(c, "/var/cache/apk"):
			n++
		case (strings.Contains(c, "yum install") || strings.Contains(c, "dnf install")) &&
			!strings.Contains(c, "clean all"):
			n++
		}
	}
	return n
}

var pipeToShell = regexp.MustCompile(`\b(curl|wget)\b[^|;&]*\|\s*(sudo\s+)?(-\S+\s+)*(ba|z|da)?sh\b`)

// PipeToShell counts steps that pipe a download straight into a shell.
func PipeToShell(h []History) int {
	n := 0
	for _, c := range runCommands(h) {
		if pipeToShell.MatchString(c) {
			n++
		}
	}
	return n
}

var secretKey = regexp.MustCompile(`(?i)(passw(or)?d|secret|token|api_?key|private_?key|access_?key|credential)`)

// SecretLookingEnv returns env var names that look like they hold secrets.
// Only names are ever inspected.
func SecretLookingEnv(keys []string) []string {
	var out []string
	for _, k := range keys {
		// *_FILE variables point at a secret, they do not contain one.
		if strings.HasSuffix(strings.ToUpper(k), "_FILE") {
			continue
		}
		if secretKey.MatchString(k) {
			out = append(out, k)
		}
	}
	return out
}

// UnpinnedInstalls counts install commands with at least one package that has no version.
func UnpinnedInstalls(h []History) int {
	n := 0
	for _, c := range runCommands(h) {
		for _, seg := range splitCommands(c) {
			if segmentUnpinned(seg) {
				n++
			}
		}
	}
	return n
}

var cmdSplit = regexp.MustCompile(`&&|\|\||;|\|`)

func splitCommands(c string) []string {
	return cmdSplit.Split(c, -1)
}

func segmentUnpinned(seg string) bool {
	f := strings.Fields(seg)
	for len(f) > 0 && (f[0] == "sudo" || strings.Contains(f[0], "=")) {
		f = f[1:] // skip sudo and leading VAR=value
	}
	if len(f) < 2 {
		return false
	}
	var pkgs []string
	var pinned func(string) bool
	switch {
	case (f[0] == "apt-get" || f[0] == "apt") && contains(f, "install"):
		pkgs = argsAfter(f, "install")
		pinned = func(p string) bool { return strings.Contains(p, "=") }
	case f[0] == "apk" && f[1] == "add":
		pkgs = argsAfter(f, "add")
		pinned = func(p string) bool { return strings.Contains(p, "=") }
	case (f[0] == "pip" || f[0] == "pip3" || strings.HasSuffix(f[0], "/pip")) && f[1] == "install",
		len(f) > 3 && strings.HasPrefix(f[0], "python") && f[1] == "-m" && f[2] == "pip" && f[3] == "install":
		pkgs = argsAfter(f, "install")
		pinned = func(p string) bool {
			return strings.Contains(p, "==") || strings.HasSuffix(p, ".txt") || strings.HasSuffix(p, ".whl") ||
				p == "." || strings.HasPrefix(p, "/") || strings.HasPrefix(p, "./")
		}
	case f[0] == "npm" && (f[1] == "install" || f[1] == "i"):
		pkgs = argsAfter(f, f[1])
		pinned = func(p string) bool {
			at := strings.LastIndex(p, "@")
			return at > 0 && at < len(p)-1 || p == "."
		}
	default:
		return false
	}
	for _, p := range pkgs {
		if !pinned(p) {
			return true
		}
	}
	return false
}

// argsAfter returns non-flag arguments after word. Flags that take a value
// (-r, -t, --target, -c, --index-url) consume the next token.
func argsAfter(f []string, word string) []string {
	i := indexOf(f, word)
	if i < 0 {
		return nil
	}
	var out []string
	valueFlags := map[string]bool{"-r": true, "--requirement": true, "-c": true, "--constraint": true,
		"-t": true, "--target": true, "-i": true, "--index-url": true, "--extra-index-url": true,
		"-o": true, "--option": true, "--repository": true, "-X": true}
	for j := i + 1; j < len(f); j++ {
		a := strings.Trim(f[j], `"'\`)
		if a == "" {
			continue
		}
		if strings.HasPrefix(a, "#") {
			break // shell comment
		}
		if valueFlags[a] {
			j++
			continue
		}
		if strings.HasPrefix(a, "-") {
			continue
		}
		out = append(out, a)
	}
	return out
}

// Installs reports whether any step installs one of the given packages.
func Installs(h []History, pkg string) bool {
	re := regexp.MustCompile(`\b(apt-get|apt|apk|yum|dnf)\b.*\b(install|add)\b.*(^|\s)` + regexp.QuoteMeta(pkg) + `(\s|=|$)`)
	for _, c := range runCommands(h) {
		for _, seg := range splitCommands(c) {
			if re.MatchString(seg) {
				return true
			}
		}
	}
	return false
}

// RunsAsRoot reports whether the default user is root.
func RunsAsRoot(user string) bool {
	u := strings.TrimSpace(user)
	if i := strings.Index(u, ":"); i >= 0 {
		u = u[:i]
	}
	return u == "" || u == "root" || u == "0"
}

func contains(f []string, s string) bool { return indexOf(f, s) >= 0 }

func indexOf(f []string, s string) int {
	for i, v := range f {
		if v == s {
			return i
		}
	}
	return -1
}

// Package facts turns raw image metadata into a Nutrition Facts report.
// It never touches the network, so every heuristic is unit testable.
package facts

import "time"

// Input is everything the heuristics need, collected by the inspect package.
type Input struct {
	Ref          string    // reference as the user typed it
	Tag          string    // tag, empty for digest references
	Digest       string    // manifest digest
	Platform     string    // os/arch[/variant]
	OS           string    // image OS from the config
	Size         int64     // sum of layer sizes in bytes
	Compressed   bool      // true when Size is the compressed download size
	Layers       int       // number of filesystem layers
	History      []History // build history, oldest first
	User         string    // Config.User
	EnvKeys      []string  // env var names only; values are never collected
	Healthcheck  bool      // a non-NONE HEALTHCHECK is set
	Created      time.Time // config created timestamp
	BaseImage    string    // from org.opencontainers.image.base.name, if any
	ExposedPorts int
}

// History is one build step.
type History struct {
	CreatedBy string
	Empty     bool
}

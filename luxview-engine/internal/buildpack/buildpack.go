package buildpack

// Buildpack defines the interface for generating Dockerfiles from detected stacks.
type Buildpack interface {
	// Name returns the human-readable name of this buildpack.
	Name() string
	// Detect returns true if this buildpack matches the given source directory.
	Detect(files []string) bool
	// Dockerfile generates the Dockerfile content for building the app.
	Dockerfile(sourceDir string) string
	// DefaultPort returns the default port the app exposes.
	DefaultPort() int
}

// All returns all available buildpacks in priority order.
// Dockerfile is checked first (highest priority), then specific frameworks, then generic.
func All() []Buildpack {
	return []Buildpack{
		&DockerfilePack{},
		&NextJsPack{},
		&VitePack{},
		&NodePack{},
		&PythonPack{},
		&GolangPack{},
		&RustPack{},
		&StaticPack{},
	}
}

// DetectStack detects the appropriate buildpack for the given files.
func DetectStack(files []string) Buildpack {
	for _, bp := range All() {
		if bp.Detect(files) {
			return bp
		}
	}
	return nil
}

// fileSet builds a quick-lookup set from a list of filenames.
func fileSet(files []string) map[string]bool {
	m := make(map[string]bool, len(files))
	for _, f := range files {
		m[f] = true
	}
	return m
}

// hasAnyPrefix checks if any file has the given prefix.
func hasAnyPrefix(files []string, prefix string) bool {
	for _, f := range files {
		if len(f) >= len(prefix) && f[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

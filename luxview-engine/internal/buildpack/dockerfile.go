package buildpack

// DockerfilePack uses the repo's own Dockerfile.
type DockerfilePack struct{}

func (d *DockerfilePack) Name() string { return "dockerfile" }

func (d *DockerfilePack) Detect(files []string) bool {
	s := fileSet(files)
	return s["Dockerfile"] || s["dockerfile"]
}

func (d *DockerfilePack) Dockerfile(_ string) string {
	// The repo already has a Dockerfile; return empty to signal "use as-is".
	return ""
}

func (d *DockerfilePack) DefaultPort() int { return 3000 }

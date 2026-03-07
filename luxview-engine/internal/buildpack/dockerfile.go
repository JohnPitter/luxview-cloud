package buildpack

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// DockerfilePack uses the repo's own Dockerfile.
type DockerfilePack struct {
	detectedPort int
}

func (d *DockerfilePack) Name() string { return "dockerfile" }

func (d *DockerfilePack) Detect(files []string) bool {
	s := fileSet(files)
	return s["Dockerfile"] || s["dockerfile"]
}

func (d *DockerfilePack) Dockerfile(_ string) string {
	// The repo already has a Dockerfile; return empty to signal "use as-is".
	return ""
}

func (d *DockerfilePack) DefaultPort() int {
	if d.detectedPort > 0 {
		return d.detectedPort
	}
	return 8080
}

// DetectPort reads the Dockerfile in sourceDir and extracts the EXPOSE port.
// Call this after detection to set the correct port.
func (d *DockerfilePack) DetectPort(sourceDir string) {
	for _, name := range []string{"Dockerfile", "dockerfile"} {
		p := filepath.Join(sourceDir, name)
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(strings.ToUpper(line), "EXPOSE ") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					// Take first port, strip /tcp or /udp suffix
					portStr := strings.Split(parts[1], "/")[0]
					if port, err := strconv.Atoi(portStr); err == nil && port > 0 {
						d.detectedPort = port
						return
					}
				}
			}
		}
		return
	}
}

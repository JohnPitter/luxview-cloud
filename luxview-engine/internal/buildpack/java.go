package buildpack

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// JavaPack detects Java/Spring Boot projects (Maven or Gradle).
type JavaPack struct{}

func (j *JavaPack) Name() string { return "java" }

func (j *JavaPack) Detect(files []string) bool {
	s := fileSet(files)
	return s["pom.xml"] || s["build.gradle"] || s["build.gradle.kts"]
}

func (j *JavaPack) Dockerfile(sourceDir string) string {
	s := fileSet(listDir(sourceDir))

	if s["build.gradle"] || s["build.gradle.kts"] {
		return j.gradleDockerfile(sourceDir, s)
	}
	return j.mavenDockerfile(sourceDir)
}

func (j *JavaPack) DefaultPort() int { return 8080 }

func (j *JavaPack) mavenDockerfile(sourceDir string) string {
	javaVersion := detectJavaVersion(sourceDir)
	hasWrapper := fileExists(filepath.Join(sourceDir, "mvnw"))

	buildCmd := "mvn"
	if hasWrapper {
		buildCmd = "./mvnw"
	}

	return fmt.Sprintf(`FROM eclipse-temurin:%s-jdk AS builder
WORKDIR /app
COPY . .
RUN if [ -f mvnw ]; then chmod +x mvnw; fi
RUN %s clean package -DskipTests -q

FROM eclipse-temurin:%s-jre
WORKDIR /app
COPY --from=builder /app/target/*.jar app.jar
ENV SERVER_PORT=%d
EXPOSE %d
CMD ["java", "-jar", "app.jar"]
`, javaVersion, buildCmd, javaVersion, j.DefaultPort(), j.DefaultPort())
}

func (j *JavaPack) gradleDockerfile(sourceDir string, files map[string]bool) string {
	javaVersion := detectJavaVersion(sourceDir)
	hasWrapper := fileExists(filepath.Join(sourceDir, "gradlew"))

	buildCmd := "gradle"
	if hasWrapper {
		buildCmd = "./gradlew"
	}

	return fmt.Sprintf(`FROM eclipse-temurin:%s-jdk AS builder
WORKDIR /app
COPY . .
RUN if [ -f gradlew ]; then chmod +x gradlew; fi
RUN %s clean bootJar -x test -q

FROM eclipse-temurin:%s-jre
WORKDIR /app
COPY --from=builder /app/build/libs/*.jar app.jar
ENV SERVER_PORT=%d
EXPOSE %d
CMD ["java", "-jar", "app.jar"]
`, javaVersion, buildCmd, javaVersion, j.DefaultPort(), j.DefaultPort())
}

// detectJavaVersion tries to detect the Java version from pom.xml or build.gradle.
// Defaults to 21 (latest LTS).
func detectJavaVersion(sourceDir string) string {
	// Check pom.xml for java.version property
	pomPath := filepath.Join(sourceDir, "pom.xml")
	if data, err := os.ReadFile(pomPath); err == nil {
		content := string(data)
		// Look for <java.version>XX</java.version>
		if idx := strings.Index(content, "<java.version>"); idx >= 0 {
			start := idx + len("<java.version>")
			end := strings.Index(content[start:], "</java.version>")
			if end > 0 {
				version := strings.TrimSpace(content[start : start+end])
				if isValidJavaVersion(version) {
					return version
				}
			}
		}
		// Look for <maven.compiler.source>XX</maven.compiler.source>
		if idx := strings.Index(content, "<maven.compiler.source>"); idx >= 0 {
			start := idx + len("<maven.compiler.source>")
			end := strings.Index(content[start:], "</maven.compiler.source>")
			if end > 0 {
				version := strings.TrimSpace(content[start : start+end])
				if isValidJavaVersion(version) {
					return version
				}
			}
		}
	}

	// Check build.gradle for sourceCompatibility or java toolchain
	for _, name := range []string{"build.gradle", "build.gradle.kts"} {
		gradlePath := filepath.Join(sourceDir, name)
		if data, err := os.ReadFile(gradlePath); err == nil {
			content := string(data)
			// Look for sourceCompatibility = '17' or JavaVersion.VERSION_17
			for _, v := range []string{"21", "17", "11"} {
				if strings.Contains(content, v) {
					return v
				}
			}
		}
	}

	return "21"
}

func isValidJavaVersion(v string) bool {
	switch v {
	case "8", "11", "17", "21":
		return true
	}
	return false
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func listDir(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

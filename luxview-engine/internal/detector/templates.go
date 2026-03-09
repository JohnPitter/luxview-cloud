package detector

import (
	"strconv"
	"strings"
)

func generateDockerfile(det Detection, repoDir string) string {
	switch det.Runtime {
	case "nodejs":
		return nodeDockerfile(det, repoDir)
	case "python":
		return pythonDockerfile(det)
	case "go":
		return goDockerfile()
	case "ruby":
		return rubyDockerfile()
	case "java":
		return javaDockerfile(det)
	case "rust":
		return rustDockerfile()
	case "static":
		return staticDockerfile()
	default:
		return nodeDockerfile(det, repoDir)
	}
}

func nodeDockerfile(det Detection, repoDir string) string {
	pm := "npm"
	lockfile := "package-lock.json"
	installCmd := "npm ci --omit=dev"
	if fileExists(repoDir, "pnpm-lock.yaml") {
		pm = "pnpm"
		lockfile = "pnpm-lock.yaml"
		installCmd = "corepack enable && pnpm install --frozen-lockfile --prod"
	} else if fileExists(repoDir, "yarn.lock") {
		pm = "yarn"
		lockfile = "yarn.lock"
		installCmd = "corepack enable && yarn install --frozen-lockfile --production"
	}

	pkg := readFile(repoDir, "package.json")
	hasBuildScript := strings.Contains(pkg, `"build"`)

	if det.Framework == "vite" {
		return "# Build stage\nFROM node:20-alpine AS builder\nWORKDIR /app\nCOPY package.json " + lockfile + " ./\nRUN " + strings.Replace(installCmd, "--prod", "", 1) + "\nCOPY . .\nRUN " + pm + " run build\n\n# Production stage\nFROM nginx:alpine\nCOPY --from=builder /app/dist /usr/share/nginx/html\nEXPOSE 80\nCMD [\"nginx\", \"-g\", \"daemon off;\"]\n"
	}

	if det.Framework == "nextjs" {
		return "FROM node:20-alpine AS builder\nWORKDIR /app\nCOPY package.json " + lockfile + " ./\nRUN " + strings.Replace(installCmd, "--prod", "", 1) + "\nCOPY . .\nRUN " + pm + " run build\n\nFROM node:20-alpine\nWORKDIR /app\nCOPY --from=builder /app/.next ./.next\nCOPY --from=builder /app/node_modules ./node_modules\nCOPY --from=builder /app/package.json ./\nCOPY --from=builder /app/public ./public\nEXPOSE 3000\nCMD [\"" + pm + "\", \"start\"]\n"
	}

	buildStep := ""
	if hasBuildScript {
		buildStep = "RUN " + pm + " run build\n"
	}

	return "FROM node:20-alpine\nWORKDIR /app\nCOPY package.json " + lockfile + " ./\nRUN " + installCmd + "\nCOPY . .\n" + buildStep + "EXPOSE " + strconv.Itoa(det.Port) + "\nCMD [\"node\", \"dist/index.js\"]\n"
}

func pythonDockerfile(det Detection) string {
	cmd := "CMD [\"python\", \"app.py\"]"
	if det.Framework == "django" {
		cmd = "CMD [\"gunicorn\", \"--bind\", \"0.0.0.0:8000\", \"config.wsgi:application\"]"
	} else if det.Framework == "fastapi" {
		cmd = "CMD [\"uvicorn\", \"main:app\", \"--host\", \"0.0.0.0\", \"--port\", \"8000\"]"
	} else if det.Framework == "flask" {
		cmd = "CMD [\"gunicorn\", \"--bind\", \"0.0.0.0:5000\", \"app:app\"]"
	}

	return "FROM python:3.12-slim\nWORKDIR /app\nCOPY requirements.txt ./\nRUN pip install --no-cache-dir -r requirements.txt\nCOPY . .\nEXPOSE " + strconv.Itoa(det.Port) + "\n" + cmd + "\n"
}

func goDockerfile() string {
	return "FROM golang:1.22-alpine AS builder\nWORKDIR /app\nCOPY go.mod go.sum ./\nRUN go mod download\nCOPY . .\nRUN CGO_ENABLED=0 go build -o /app/server .\n\nFROM alpine:3.19\nWORKDIR /app\nCOPY --from=builder /app/server .\nEXPOSE 8080\nCMD [\"./server\"]\n"
}

func rubyDockerfile() string {
	return "FROM ruby:3.3-slim\nWORKDIR /app\nCOPY Gemfile Gemfile.lock ./\nRUN bundle install --without development test\nCOPY . .\nEXPOSE 3000\nCMD [\"bundle\", \"exec\", \"rails\", \"server\", \"-b\", \"0.0.0.0\"]\n"
}

func javaDockerfile(det Detection) string {
	if det.Framework == "gradle" {
		return "FROM gradle:8-jdk21-alpine AS builder\nWORKDIR /app\nCOPY . .\nRUN gradle build --no-daemon -x test\n\nFROM eclipse-temurin:21-jre-alpine\nWORKDIR /app\nCOPY --from=builder /app/build/libs/*.jar app.jar\nEXPOSE 8080\nCMD [\"java\", \"-jar\", \"app.jar\"]\n"
	}
	return "FROM maven:3.9-eclipse-temurin-21-alpine AS builder\nWORKDIR /app\nCOPY . .\nRUN mvn package -DskipTests\n\nFROM eclipse-temurin:21-jre-alpine\nWORKDIR /app\nCOPY --from=builder /app/target/*.jar app.jar\nEXPOSE 8080\nCMD [\"java\", \"-jar\", \"app.jar\"]\n"
}

func rustDockerfile() string {
	return "FROM rust:1.77-alpine AS builder\nWORKDIR /app\nRUN apk add --no-cache musl-dev\nCOPY . .\nRUN cargo build --release\n\nFROM alpine:3.19\nWORKDIR /app\nCOPY --from=builder /app/target/release/* .\nEXPOSE 8080\nCMD [\"./app\"]\n"
}

func staticDockerfile() string {
	return "FROM nginx:alpine\nCOPY . /usr/share/nginx/html\nEXPOSE 80\nCMD [\"nginx\", \"-g\", \"daemon off;\"]\n"
}

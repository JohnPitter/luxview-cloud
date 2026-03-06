package buildpack

import "fmt"

// GolangPack detects Go projects.
type GolangPack struct{}

func (g *GolangPack) Name() string { return "go" }

func (g *GolangPack) Detect(files []string) bool {
	s := fileSet(files)
	return s["go.mod"]
}

func (g *GolangPack) Dockerfile(_ string) string {
	return fmt.Sprintf(`FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /server ./...

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /server .
EXPOSE %d
CMD ["./server"]
`, g.DefaultPort())
}

func (g *GolangPack) DefaultPort() int { return 8080 }

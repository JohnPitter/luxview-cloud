package buildpack

import "fmt"

// RustPack detects Rust projects.
type RustPack struct{}

func (r *RustPack) Name() string { return "rust" }

func (r *RustPack) Detect(files []string) bool {
	s := fileSet(files)
	return s["Cargo.toml"]
}

func (r *RustPack) Dockerfile(_ string) string {
	return fmt.Sprintf(`FROM rust:1.77-slim AS builder
WORKDIR /app
COPY Cargo.toml Cargo.lock* ./
# Create dummy main to cache dependencies
RUN mkdir src && echo "fn main() {}" > src/main.rs
RUN cargo build --release
RUN rm -rf src
COPY . .
RUN touch src/main.rs && cargo build --release

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=builder /app/target/release/* ./
EXPOSE %d
CMD ["./app"]
`, r.DefaultPort())
}

func (r *RustPack) DefaultPort() int { return 8080 }

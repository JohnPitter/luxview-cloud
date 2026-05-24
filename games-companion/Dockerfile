FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod tidy && go build -o games-companion .

FROM alpine:latest
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/games-companion /games-companion
EXPOSE 8080
ENTRYPOINT ["/games-companion"]

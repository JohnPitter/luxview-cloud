FROM golang:alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /engine ./cmd/engine

FROM alpine:3.20

RUN apk --no-cache add ca-certificates git docker-cli wget

# Download GeoIP database
COPY scripts/download-geolite2.sh /tmp/download-geolite2.sh
RUN chmod +x /tmp/download-geolite2.sh && /tmp/download-geolite2.sh /usr/share/GeoIP

WORKDIR /app
COPY --from=builder /engine .

EXPOSE 8080

CMD ["./engine"]

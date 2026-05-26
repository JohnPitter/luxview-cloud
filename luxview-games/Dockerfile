FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod tidy && go build -o luxview-games .

FROM alpine:latest
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/luxview-games /luxview-games
EXPOSE 8080
ENTRYPOINT ["/luxview-games"]

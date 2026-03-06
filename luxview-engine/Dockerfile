FROM golang:1.22-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /engine ./cmd/engine

FROM alpine:3.19

RUN apk --no-cache add ca-certificates git

WORKDIR /app
COPY --from=builder /engine .

EXPOSE 8080

CMD ["./engine"]

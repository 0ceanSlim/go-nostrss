FROM golang:1.23-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o go-nostrss .

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/go-nostrss .
VOLUME ["/config"]
WORKDIR /config
CMD ["/app/go-nostrss"]


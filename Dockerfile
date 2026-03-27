FROM golang:alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o syncer-app ./cmd/syncer

FROM alpine:latest
WORKDIR /app

COPY --from=builder /app/syncer-app .
COPY --from=builder /app/config ./config

ENV CONFIG_PATH=./config/local.yaml

EXPOSE 8888

CMD ["./syncer-app"]
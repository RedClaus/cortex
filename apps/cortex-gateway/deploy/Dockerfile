FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -o cortex-gateway ./cmd/cortex-gateway

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/cortex-gateway .

COPY config.yaml .

CMD ["./cortex-gateway"]

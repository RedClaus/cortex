# Build stage
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o cortex-gateway ./cmd/cortex-gateway/

# Runtime stage
FROM alpine:3.21
RUN apk --no-cache add ca-certificates tzdata wget
WORKDIR /app
COPY --from=builder /app/cortex-gateway .
COPY --from=builder /app/web ./web/
EXPOSE 18800 18793
VOLUME ["/app/data"]
ENV GATEWAY_PORT=18800
ENTRYPOINT ["./cortex-gateway"]

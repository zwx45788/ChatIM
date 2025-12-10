# Build Stage
FROM golang:1.25.1-alpine AS builder

WORKDIR /app
ENV GOPROXY=https://goproxy.cn,direct

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# 构建 message-service 二进制文件

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o message-service ./cmd/message

# Final Stage
FROM alpine:latest

WORKDIR /root/

COPY --from=builder /app/message-service .
COPY --from=builder /app/pkg/config/config.yaml ./config/config.yaml
COPY --from=builder /app/migrations ./migrations

# 暴露 gRPC 端口
EXPOSE 50052

CMD ["./message-service"]

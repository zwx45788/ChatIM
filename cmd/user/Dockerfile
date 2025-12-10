# Build Stage
FROM golang:1.25.1-alpine AS builder

WORKDIR /app

ENV GOPROXY=https://goproxy.cn,direct
# 复制 go mod 文件并下载依赖
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .
# 构建 user-service 二进制文件

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o user-service ./cmd/user

# Final Stage
FROM alpine:latest

WORKDIR /root/

# 从 builder 阶段复制二进制文件
COPY --from=builder /app/user-service .

# 复制配置文件到 config 目录
COPY --from=builder /app/pkg/config/config.yaml ./config/config.yaml

# 复制迁移文件
COPY --from=builder /app/migrations ./migrations

# 暴露 gRPC 端口
EXPOSE 50051

CMD ["./user-service"]

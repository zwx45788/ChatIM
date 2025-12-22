# 第一阶段：构建
FROM golang:1.25.1-alpine AS builder

WORKDIR /app

ENV GOPROXY=https://goproxy.cn,direct
# 安装必要的工具
RUN apk add --no-cache git

# 复制 go.mod 和 go.sum
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制整个源代码
COPY . .

# 编译 Group Service
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o group-service ./cmd/group/main.go

# 第二阶段：运行
FROM alpine:latest

WORKDIR /root/

# 从构建阶段复制编译好的二进制文件
COPY --from=builder /app/group-service .

# 复制配置文件
COPY --from=builder /app/pkg/config/config.yaml ./config/config.yaml

# 复制迁移文件
COPY --from=builder /app/migrations ./migrations

# 暴露端口（GroupService 的 gRPC 端口）
EXPOSE 50053

# 启动服务
CMD ["./group-service"]

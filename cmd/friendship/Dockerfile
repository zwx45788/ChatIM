# 构建阶段
FROM golang:1.21-alpine AS builder

WORKDIR /build

# 安装必要的工具
RUN apk add --no-cache git

# 复制 go.mod 和 go.sum
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建二进制文件
RUN CGO_ENABLED=0 GOOS=linux go build -o friendship ./cmd/friendship

# 运行阶段
FROM alpine:latest

# 安装 ca-certificates 用于 HTTPS
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# 从构建阶段复制二进制文件
COPY --from=builder /build/friendship .

# 复制配置文件
COPY pkg/config/config.yaml ./config/

# 暴露端口
EXPOSE 50054

# 启动应用
CMD ["./friendship"]

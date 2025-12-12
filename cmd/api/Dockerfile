# Build Stage
FROM golang:1.25.1-alpine AS builder

WORKDIR /app

ENV GOPROXY=https://goproxy.cn,direct
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# 构建 api-gateway 二进制文件

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o api-gateway ./cmd/api

# Final Stage
FROM alpine:latest

WORKDIR /root/

COPY --from=builder /app/api-gateway .
COPY --from=builder /app/pkg/config/config.yaml ./config/config.yaml
COPY --from=builder /app/frontend-wechat-enterprise ./web

# 暴露 HTTP 端口
EXPOSE 8080

CMD ["./api-gateway"]

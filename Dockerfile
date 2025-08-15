# 多阶段构建的Dockerfile
# 用于构建Linux服务管理器

# 构建阶段
FROM golang:1.21-alpine AS builder

# 安装必要的包
RUN apk add --no-cache git ca-certificates tzdata

# 设置工作目录
WORKDIR /app

# 复制go mod文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o mcp-server ./cmd/server

# 运行测试
RUN go test -v ./pkg/... ./internal/... ./cmd/...

# 运行阶段
FROM scratch

# 从builder阶段复制时区数据
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# 从builder阶段复制CA证书
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# 从builder阶段复制二进制文件
COPY --from=builder /app/mcp-server /mcp-server

# 创建非root用户（在scratch镜像中需要手动创建）
USER 65534:65534

# 暴露端口
EXPOSE 8080

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD ["/mcp-server", "-version"]

# 设置入口点
ENTRYPOINT ["/mcp-server"]

# 默认以HTTP模式运行
CMD ["-config", "/config/config.yaml"]

# 标签
LABEL maintainer="Linux Service Manager Team"
LABEL description="Linux Service Manager - AI-driven service management tool"
LABEL version="1.0.0"
# 构建阶段 - 使用 Debian 基础的 Go 镜像
FROM golang:1.21-bullseye AS builder

# 设置工作目录
WORKDIR /app

# 设置国内源（清华大学源）
RUN sed -i 's/deb.debian.org/mirrors.tuna.tsinghua.edu.cn/g' /etc/apt/sources.list && \
    sed -i 's/security.debian.org/mirrors.tuna.tsinghua.edu.cn/g' /etc/apt/sources.list

# 安装必要的构建工具
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    tzdata \
    gcc \
    musl-dev \
    && rm -rf /var/lib/apt/lists/*

# 设置 GOPROXY 和 GO111MODULE
ENV GOPROXY=https://goproxy.cn,direct
ENV GO111MODULE=on
ENV CGO_ENABLED=1

# 复制 go mod 文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
RUN GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o opensearch-alert \
    ./cmd/alert

# 运行阶段
FROM alpine:3.18

# 设置 Alpine Linux 国内源
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories && \
    apk --no-cache add \
    ca-certificates \
    tzdata \
    curl \
    procps \
    && rm -rf /var/cache/apk/*

# 设置时区
RUN cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone

# 创建非root用户和组
RUN addgroup -g 1001 -S alert && \
    adduser -u 1001 -S alert -G alert

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/opensearch-alert /usr/local/bin/opensearch-alert

# 复制配置文件
COPY --from=builder /app/configs ./configs

COPY --from=builder /app/web ./web

# 创建日志目录
RUN mkdir -p /var/log/opensearch-alert && \
    chown -R alert:alert /app/

# 切换到非root用户
USER alert

# 设置环境变量
ENV TZ=Asia/Shanghai

# 健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD ps aux | grep opensearch-alert | grep -v grep > /dev/null || exit 1

# 启动命令
CMD ["/usr/local/bin/opensearch-alert", "-config=/app/configs/config.yaml"]
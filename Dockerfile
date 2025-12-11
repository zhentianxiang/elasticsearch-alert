FROM golang:1.21-alpine AS builder

# 使用国内 Go 模块代理，加速依赖下载
ENV GOPROXY=https://goproxy.cn,direct \
    GO111MODULE=on

# 替换为阿里云 Alpine 源，加速 apk
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories

WORKDIR /src

# 先复制 go.mod/go.sum 并预先下载依赖
COPY go.mod go.sum ./

RUN go mod download

# 再复制项目源码
COPY . .

# 删除 --mount 参数
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/elasticsearch-alert ./cmd/alert

FROM alpine:3.18

# 使用国内镜像源并安装基础依赖和时区数据
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories \
    && apk add --no-cache ca-certificates tzdata \
    && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo "Asia/Shanghai" > /etc/timezone

WORKDIR /app

COPY --from=builder /out/elasticsearch-alert /app/elasticsearch-alert

USER root

ENTRYPOINT ["/app/elasticsearch-alert"]
CMD ["-config", "/app/configs/config.yaml"]

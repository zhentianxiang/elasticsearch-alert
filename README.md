# Elasticsearch Alert（仅后端，无前端/无MySQL）

Elasticsearch Alert 是一个面向日志的轻量告警工具：按规则定时查询 Elasticsearch，在命中阈值时通过通知渠道（控制台/Webhook/飞书）发送告警。该目录是一个独立的 Go 子项目，不依赖本仓库中 OpenSearch 版本的前后端与 MySQL。

## 功能特性

- 基于 YAML 定义规则：索引、查询、时间窗、阈值、调度（cron）
- 支持查询命中统计与样例事件回显（前 N 条）
- 去重与静默期，避免重复告警
- 通知渠道：控制台、通用 Webhook、飞书（支持 @all）
- 单二进制部署，亦提供 Dockerfile

## 快速开始

1) 配置

将示例配置复制为实际配置：

```bash
cp ./configs/config.example.yaml ./configs/config.yaml
cp -r ./configs/rules.example ./configs/rules
```

根据环境修改 `configs/config.yaml` 中的 Elasticsearch 与通知参数，并在 `configs/rules` 下按需增删规则文件。

2) 本地运行

```bash
cd elasticsearch-alert
go build -o bin/elasticsearch-alert ./cmd/alert
./bin/elasticsearch-alert -config ./configs/config.yaml
```

3) Docker 运行

```bash
cd elasticsearch-alert
docker build -t elasticsearch-alert:latest .
docker run --rm -v $(pwd)/configs:/app/configs:ro elasticsearch-alert:latest -config /app/configs/config.yaml
```

## 配置说明

- `configs/config.yaml`：全局配置（ES、通知、时区、规则目录、静默期等）
- `configs/rules/*.yaml`：规则文件，示例见 `configs/rules.example/`

示例（节选）：

```yaml
elasticsearch:
  addresses: ["http://localhost:9200"]
  username: ""
  password: ""
  tlsSkipVerify: false

scheduler:
  timezone: "Asia/Shanghai"

rules:
  directory: "./configs/rules"
  sampleSize: 3
  defaultQuietPeriod: "5m"

notifications:
  webhook:
    url: ""
    headers: {}
  feishu:
    webhook: ""
    enableAtAll: true
  dingtalk:
    webhook: ""
    enableAtAll: true
  wechat:
    webhook: ""
  email:
    host: ""
    port: 587
    username: ""
    password: ""
    from: "alert@example.com"
    to: ["dev@example.com"]
```

规则示例（节选）：

```yaml
name: "High Error Rate"
description: "5分钟内 ERROR 日志条数超过阈值"
index: "logs-*"
cron: "*/5 * * * *"          # 每5分钟执行
timeWindow: "5m"
queryString: 'level:(ERROR OR FATAL)'
threshold:
  countGt: 100
dedup:
  quietPeriod: "10m"
alerts:
  channels: ["feishu", "webhook", "dingtalk", "wechat", "email"]
```

## 目录结构

- `cmd/alert`：入口
- `internal/config`：配置与规则加载
- `internal/elasticsearch`：ES 客户端封装
- `internal/alert`：规则模型、告警引擎与调度
- `internal/notification`：通知发送实现（控制台、Webhook、飞书）
  - 支持：`console`、`webhook`、`feishu`、`dingtalk`、`wechat`、`email`
- `configs/`：示例配置与规则

## 说明

- 本服务不包含前端页面与 MySQL 存储，仅作为日志告警工具运行。
- 飞书通知示例支持 @all，可按需关闭或扩展其他渠道。



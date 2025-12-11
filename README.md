# Elasticsearch Alert 日志监控告警插件

Elasticsearch Alert 是一个面向 Kubernetes / 日志场景的轻量告警工具：按规则定时查询 Elasticsearch，在命中阈值时通过多种通知渠道发送富文本告警。

## 功能特性

- 基于 YAML 定义规则：索引、查询 DSL、时间窗、阈值、调度（秒级 cron）
- 支持查询命中统计与样例事件（按 `@timestamp desc` 取前 N 条）
- 去重与静默期（`dedup.quietPeriod`），避免重复骚扰
- 通知渠道：控制台、通用 Webhook、飞书、钉钉、企业微信、邮箱
- 飞书 / 钉钉 / 企业微信 / 邮件统一美观模板：标题 Emoji、摘要信息、节点/命名空间/Pod/镜像/错误日志
- 单二进制部署，提供国内友好的 Dockerfile 与 docker-compose
- 支持直接对接旧版 Elasticsearch 7.x（通过 `provider: opensearch` + `skipProductCheck`）

## 快速开始

### 1) 配置

根据环境修改 `configs/config.yaml` 中的 Elasticsearch 与通知参数，并在 `configs/rules` 下按需增删规则文件。

当前仓库已经自带一个基础规则示例 `configs/rules/k8s-error-all.yaml`，可直接用于验证。

### 2) 本地运行

```bash
cd elasticsearch-alert
go build -o bin/elasticsearch-alert ./cmd/alert
./bin/elasticsearch-alert -config ./configs/config.yaml
```

### 3) 使用 Docker / docker-compose 运行

构建镜像（已内置国内加速）：

```bash
cd elasticsearch-alert
docker build -t elasticsearch-alert:latest .
```

使用 docker-compose 运行并挂载配置：

```bash
docker-compose up -d
```

默认会把当前目录下的 `./configs` 挂载到容器内的 `/app/configs`，程序入口参数为 `-config /app/configs/config.yaml`。

## 构建与交叉编译

本项目是标准 Go 程序，支持一次源码、多平台构建。下面是常用的交叉编译示例（在项目根目录执行）：

```bash
# 当前平台构建
go build -o bin/elasticsearch-alert ./cmd/alert

# Linux amd64（服务器常用）
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/elasticsearch-alert-linux-amd64 ./cmd/alert

# Linux arm64（部分 ARM 服务器 / K8s 节点）
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/elasticsearch-alert-linux-arm64 ./cmd/alert

# macOS amd64（Intel Mac）
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o bin/elasticsearch-alert-darwin-amd64 ./cmd/alert

# macOS arm64（Apple Silicon）
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o bin/elasticsearch-alert-darwin-arm64 ./cmd/alert

# Windows amd64
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o bin/elasticsearch-alert-windows-amd64.exe ./cmd/alert
```

生成的二进制可以直接放到对应主机或容器中运行，命令行参数保持一致：

```bash
./elasticsearch-alert -config ./configs/config.yaml
```

## 配置说明

- `configs/config.yaml`：全局配置（ES、通知、时区、规则目录、静默期等）
- `configs/rules/*.yaml`：规则文件，每个文件对应一条独立告警规则

示例（节选）：

```yaml
elasticsearch:
  addresses: ["http://localhost:30900"]
  username: ""
  password: ""
  tlsSkipVerify: false
  requestTimeout: "30s"
  # 对接 Elasticsearch 7.x 建议：
  provider: "opensearch"      # 使用 OpenSearch Go SDK，兼容旧版 ES
  skipProductCheck: true      # 跳过 X-Elastic-Product 检查

scheduler:
  timezone: "Asia/Shanghai"

rules:
  directory: "./configs/rules"
  sampleSize: 3
  defaultQuietPeriod: "10m"

notifications:
  webhook:
    url: ""
    headers: {}
    timeout: "5s"
  feishu:
    webhook: ""
    enableAtAll: true
    timeout: "5s"
    titlePrefix: "[日志告警]"
    contentIntro: "检测到规则触发，以下为摘要与日志详情："
  dingtalk:
    webhook: ""
    secret: ""                # 选填，启用“加签”时必填
    enableAtAll: true
    timeout: "5s"
  wechat:
    webhook: ""
    timeout: "5s"
  email:
    host: ""
    port: 587
    username: ""
    password: ""
    from: "alert@example.com"
    to: ["dev@example.com"]
    useTLS: false
    tlsSkipVerify: false
    subjectPrefix: "[Log Alert]"
    timeout: "10s"
```

规则示例（节选）：

```yaml
name: "系统 ERROR 日志告警（全索引）"
description: "监控所有 k8s-app-* 索引中的 ERROR 级别应用日志，用于快速发现严重错误"
index: "k8s-app-*"
cron: "0 */1 * * * *"        # 每分钟执行一次
timeWindow: "5m"

# 告警级别，仅用于通知展示
severity: "High"             # 支持：Critical / High / Medium / Low / Info

# 查询 DSL：message 中包含错误关键词
dsl:
  bool:
    filter:
      - query_string:
          query: 'message: "*ERROR*" OR message: "*Exception*" OR message: "*Throwable*"'
          analyze_wildcard: true

threshold:
  countGt: 10                # 最近 5 分钟内命中条数 > 10 触发告警

dedup:
  quietPeriod: "10m"         # 10 分钟内同一规则只告警一次

alerts:
  channels: ["feishu", "dingtalk", "wechat", "email", "console"]
```

## 日志索引与字段要求（如何查看 _mapping）

告警正文中会从命中的日志里抽取一些字段，用于展示“本次告警目标”与“错误日志”，如果字段不存在，则对应信息会为空。建议索引中至少包含以下字段（名称可以通过 ingest/日志采集配置控制）：

- `@timestamp`：日志时间（`date` 类型）
- `message`：日志原文（`text` / `keyword`）
- `kubernetes_host`：节点名称
- `kubernetes_namespace_name`：命名空间
- `kubernetes_pod_name`：Pod 名称
- `kubernetes_container_image`：容器镜像
- `kubernetes_labels_app` + `kubernetes_labels_app.keyword`：应用标识标签（用于 DSL 中 `term` 过滤）

可以通过如下方式查看当前索引的字段结构（mapping）：

```bash
curl -X GET "http://<es-host>:9200/k8s-app-*/_mapping?pretty"
```

重点检查：

- 上面列出的字段是否存在；
- 需要精确匹配的字段（如 `kubernetes_labels_app`）是否有 `.keyword` 子字段，规则中的 `term: kubernetes_labels_app.keyword: "xxx"` 需要该字段为 `keyword` 类型。

如果你的字段名与上述不一致（例如使用其他采集器或字段前缀），只需要：

- 在规则 DSL 中改成你自己的字段名；
- 确保 `internal/alert/engine.go` 中使用的展示字段（节点 / 命名空间 / Pod / 镜像 / message / `@timestamp`）与你的实际字段一致，或者按需修改那几个字段名。

## 目录结构

- `cmd/alert`：入口
- `internal/config`：配置与规则加载
- `internal/elasticsearch`：ES 客户端封装（支持 provider / 跳过产品检查）
- `internal/alert`：规则模型、告警引擎与调度、告警正文渲染（含 severity / 样例抽取）
- `internal/notification`：通知发送实现
  - 支持：`console`、`webhook`、`feishu`、`dingtalk`（支持 secret 加签）、`wechat`、`email`
- `configs/`：配置与规则

## 最近更新要点

- 新增规则字段 `severity`，用于在通知模板中展示告警级别（不影响查询逻辑）
- 优化告警正文渲染：统一包含告警概览、本次告警目标（节点 / Namespace / Pod / 镜像）、错误日志内容
- 飞书改为交互式卡片（`msg_type=interactive`），钉钉 / 企业微信改为 Markdown 模板，邮箱改为 HTML 卡片样式
- 钉钉通知支持 `secret` 加签，并解析 `errcode/errmsg`，日志中可看到具体失败原因
- Elasticsearch 客户端增加 `provider` / `skipProductCheck` 支持，方便对接旧版 Elasticsearch 7.x 或代理层
- Dockerfile 使用国内镜像源与 Go 模块代理，加速构建；新增 `docker-compose.yml` 方便挂载配置一键启动

## 说明

- 飞书 / 钉钉 / 企业微信 / 邮件模板已经适配生产使用，建议在规则中合理设置阈值与静默期，避免告警风暴。


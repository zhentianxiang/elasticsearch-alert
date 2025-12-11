# KubeSphere-OpenSearch 告警系统

一款基于 Go 的 OpenSearch 告警与可视化系统，支持 Web 管理台、规则管理、配置管理、历史告警查看及多种通知渠道。内置 SQLite/MySQL 双数据库支持，可在 Kubernetes 环境以多副本运行，内置分布式锁与去重，避免重复告警。

## 功能总览

- 多数据源：对接 OpenSearch（日志、事件、审计）。
- 灵活规则：frequency/any/spike/flatline 等，YAML 文件配置，可启用/禁用，Web 可视化编辑与持久化。
- 通知渠道：Email、钉钉、企业微信、飞书。
- 告警级别：Critical/High/Medium/Low/Info，支持自动判断与自定义。
- 告警抑制：固定间隔与指数级抑制；跨副本共享（后续可持续增强）。
- 历史与仪表盘：Dashboard 图表（级别分布/时间趋势/总量）、列表分页、详情与原始消息展示。
- 安全：登录会话、RBAC（admin/viewer）、密码不回传；XSS 转义。
- 多数据库：SQLite（默认）/MySQL 8.0+，Session 与告警历史持久化。
- 多副本：规则级分布式锁、发送前去重、状态持久化（进行中）。

## 目录结构

```
opensearch-alert/
├── cmd/alert/                  # 主程序入口
├── internal/
│   ├── alert/                  # 告警引擎（规则执行、触发、写回 OS、分布式锁/去重）
│   ├── config/                 # 配置加载与规则加载
│   ├── database/               # 数据库抽象（SQLite/MySQL）
│   ├── notification/           # 通知渠道（email/dingtalk/wechat/feishu）
│   ├── opensearch/             # OpenSearch 客户端
│   └── web/                    # Web 服务端（API、模板、静态资源）
├── pkg/types/                  # 公共类型定义
├── configs/
│   ├── config.yaml             # 主配置（OpenSearch、通知、日志、Web、数据库、鉴权、规则默认）
│   └── rules/                  # 规则文件目录（*.yaml）
├── web/
│   ├── templates/              # 页面模板（dashboard/alerts/rules/config/login）
│   └── static/                 # JS/CSS 资源
├── k8s/                        # Kubernetes 清单（ConfigMap/Deployment/Service/RBAC）
├── Dockerfile                  # 构建镜像
└── WEB_DASHBOARD.md            # Web 控制台说明
```

## 构建与运行

### 本地运行
```bash
go mod tidy
go run cmd/alert/main.go -config=configs/config.yaml
```

### Docker
```bash
docker build -t opensearch-alert:latest .
docker run --rm -p 8080:8080 \
  -v $(pwd)/configs:/app/configs \
  -v $(pwd)/data:/app/data \
  -e INSTANCE_ID=$(hostname) \
  opensearch-alert:latest
```

### Kubernetes
```bash
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/service.yaml
```
- 多副本运行：Deployment `replicas > 1`，建议：
  - MySQL：各副本共享同库即可。
  - SQLite：使用 PVC 共享同一 DB 文件（路径见 `configs/config.yaml` 的 `database.path`）。
  - 设置环境变量 `INSTANCE_ID`（可使用 Downward API 注入 Pod 名）。

示例（在 Deployment 容器 env 中）：
```yaml
- name: INSTANCE_ID
  valueFrom:
    fieldRef:
      fieldPath: metadata.name
```

## 配置说明（configs/config.yaml）

关键字段摘要（按实际文件为准）：
- opensearch：主机、端口、协议、认证、证书校验、超时。
- alert_engine：
  - run_interval: 规则运行周期（秒）
  - buffer_time: 查询时间缓冲（秒）
  - writeback_index: 写回 OpenSearch 的索引名
  - alert_time_limit: 告警时间窗口限制（秒）
  - lock_ttl_seconds（可选，待加入）：分布式锁 TTL
  - dedupe_ttl_seconds（可选，待加入）：发送去重 TTL
- alert_suppression：是否开启、固定间隔、指数级抑制参数。
- notifications：Email/DingTalk/WeChat/Feishu 开关与凭据。
- logging：级别、格式、文件、滚动策略。
- web：监听、静态路径、模板路径、会话密钥等。
- database：
  - type: sqlite | mysql
  - SQLite: path、连接池
  - MySQL: host/port/username/password/dbname/params（默认含 `charset=utf8mb4&parseTime=true&loc=Local`）
- auth：开关、会话超时、用户列表（admin/viewer）。
- rules：规则目录、默认时间窗/阈值。

## 规则文件（configs/rules/*.yaml）

统一格式示例：
```yaml
name: "应用Pod警告日志告警"
type: "frequency"          # frequency|any|spike|flatline
index: "ks-whizard-logging-*"
threshold: 1                # 触发阈值
timeframe: 300              # 秒
query:                      # OpenSearch DSL 片段
  bool:
    must:
      - term: { level: "WARNING" }
      - match: { message: "error|fail|warn" }
alert:
  - "feishu"
level: "Medium"            # 可选；不填将自动判断
enabled: true
```

## Web 管理台
- Dashboard：总量、级别分布、时间趋势、活跃规则数。
- 告警列表：分页、筛选、查看详情（含原始 message 转义显示）。
- 规则管理：启用/禁用、编辑保存（落盘到 rules/*.yaml），阈值即时刷新，RBAC 校验。
- 配置管理：查看与编辑（持久化到 `configs/config.yaml`），MySQL/SQLite 字段动态显示。
- 登录/RBAC：`admin` 可写、`viewer` 只读；认证信息不回传（密码字段不序列化）。
- UI 优化：统一按钮样式、配色对比度提升、页脚版权。

## 多副本支持（重要）
- 分布式锁（`rule_locks` 表）：
  - 按规则名租约锁；仅持有锁的副本执行该规则；TTL 默认 30 秒。
  - 键字段：`rule_name, locked_by, locked_at, ttl_seconds`。
- 发送前去重（`alert_dedupe` 表）：
  - 去重键：`(rule_name, level, SHA1(message))`。
  - 在 TTL（默认 120s）内已发送则跳过发送与落库。
- 历史写库（`alert_history` 表）：
  - 用于 Dashboard/列表/详情/统计；与发送链路解耦。

## 常用 MySQL 查询

```sql
-- 设置客户端、连接、结果集字符集
SET NAMES utf8mb4;

-- 查询最近100条
SELECT DATE_FORMAT(timestamp,'%Y-%m-%d %H:%i:%s') AS ts, rule_name, level, message, count
FROM alert_history
ORDER BY timestamp DESC
LIMIT 100;

-- 查询24小时内的
SELECT DATE_FORMAT(timestamp,'%H') AS hour, COUNT(*) AS cnt
FROM alert_history
WHERE timestamp >= NOW() - INTERVAL 24 HOUR
GROUP BY hour
ORDER BY hour;

-- 查询告警级别出现的次数
SELECT level, COUNT(*) AS cnt
FROM alert_history
GROUP BY level;

-- 最近告警，便于人工核对
SELECT DATE_FORMAT(timestamp,'%Y-%m-%d %H:%i:%s') AS ts, rule_name, level, LEFT(message, 120) AS msg, count
FROM alert_history
ORDER BY timestamp DESC
LIMIT 50;

-- 去重表：最近一次发送的签名与时间
SELECT rule_name, level, message_hash, DATE_FORMAT(last_sent,'%Y-%m-%d %H:%i:%s') AS last_sent, ttl_seconds
FROM alert_dedupe
ORDER BY last_sent DESC
LIMIT 20;

-- 锁表：查看谁持有锁；age_seconds > ttl_seconds 说明锁已过期
SELECT rule_name, locked_by,
       TIMESTAMPDIFF(SECOND, locked_at, NOW()) AS age_seconds,
       ttl_seconds
FROM rule_locks
ORDER BY locked_at DESC
LIMIT 50;

-- 检查近10分钟是否有重复发送（同规则/级别/消息）
SELECT rule_name, level, SHA1(message) AS msg_hash, COUNT(*) AS cnt,
       MIN(timestamp) AS first_ts, MAX(timestamp) AS last_ts
FROM alert_history
WHERE timestamp >= NOW() - INTERVAL 10 MINUTE
GROUP BY rule_name, level, msg_hash
HAVING cnt > 1
ORDER BY cnt DESC;

-- 针对某个 message_hash 验证在 TTL(120s) 内是否只落一条
SELECT DATE_FORMAT(timestamp,'%Y-%m-%d %H:%i:%s') AS ts, alert_id, rule_name, level,
       LEFT(message, 120) AS msg
FROM alert_history
WHERE rule_name='应用Pod警告日志告警' AND level='Medium'
  AND SHA1(message)='09b01e1f2b944249937a83afd8c6f494d3f4a5b6'
ORDER BY timestamp DESC
LIMIT 5;

-- 观察当前持有锁的副本（执行窗口内跑这条）
SELECT rule_name, locked_by,
       TIMESTAMPDIFF(SECOND, locked_at, NOW()) AS age_seconds, ttl_seconds
FROM rule_locks
WHERE locked_by <> ''
ORDER BY locked_at DESC
LIMIT 50;
```

## SQLite 查询示例

```sql
-- 最近100条
SELECT strftime('%Y-%m-%d %H:%M:%S', timestamp, 'localtime') AS ts, rule_name, level, message, count
FROM alert_history
ORDER BY timestamp DESC
LIMIT 100;

-- 24小时内的
SELECT strftime('%H', timestamp, 'localtime') AS hour, COUNT(*) AS cnt
FROM alert_history
WHERE timestamp >= datetime('now','-24 hours')
GROUP BY hour
ORDER BY hour;

-- 级别统计
SELECT level, COUNT(*) AS cnt FROM alert_history GROUP BY level;

-- 去重表/锁表（类似 MySQL，注意时间函数差异）
SELECT rule_name, level, message_hash, last_sent, ttl_seconds FROM alert_dedupe ORDER BY last_sent DESC LIMIT 20;
SELECT rule_name, locked_by, locked_at, ttl_seconds FROM rule_locks ORDER BY locked_at DESC LIMIT 50;
```

## 安全与 RBAC
- 仅 `admin` 角色可编辑配置/规则、启用/禁用规则。
- `viewer` 只读；前后端均校验。
- `/api/auth/check` 不返回明文密码；后端结构体已通过 `json:"-"` 屏蔽密码字段。
- 前端显示原始 message 时进行 HTML 转义，降低 XSS 风险。

## 日志与排障
- 统一日志格式；规则加载日志降为 debug；控制台更干净。
- 如时间趋势轴不准，确认 DB 查询使用本地时区（已修正 SQLite `strftime('localtime')` 与 MySQL `DATE_FORMAT`）。
- MySQL 字符集：确保 `charset=utf8mb4`，客户端可 `SET NAMES utf8mb4`。

## 版本与变更要点（近期）
- 新增 MySQL 8.0+ 支持（连接、表结构、索引创建差异、会话 UPSERT）。
- Web 管理完善：规则启停/编辑、配置编辑持久化、分页、详情原文展示。
- 安全修复：不回传密码、RBAC 生效。
- 多副本能力：规则级锁（`rule_locks`）、发送前去重（`alert_dedupe`）。

## 许可证
MIT License

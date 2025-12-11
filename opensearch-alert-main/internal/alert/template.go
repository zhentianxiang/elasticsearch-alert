package alert

import (
	"encoding/json"
	"fmt"
	"opensearch-alert/pkg/types"
	"regexp"
	"strings"
	"time"
)

// TemplateEngine 模板引擎
type TemplateEngine struct{}

// NewTemplateEngine 创建模板引擎
func NewTemplateEngine() *TemplateEngine {
	return &TemplateEngine{}
}

// BuildAlertMessage 根据事件类型构建告警消息
func (te *TemplateEngine) BuildAlertMessage(rule types.AlertRule, response *types.OpenSearchResponse) string {
	// 根据索引类型确定事件类型
	eventType := te.detectEventType(rule.Index)

	// 若设置了自定义模板，则在自定义文本后追加系统默认详情，避免信息过少
	if rule.AlertText != "" {
		custom := te.buildCustomAlertMessage(rule, response)
		var details string
		switch eventType {
		case "events":
			details = te.buildEventAlertMessage(rule, response)
		case "logging":
			if strings.Contains(rule.Name, "系统组件") {
				details = te.buildSystemComponentLoggingAlertMessage(rule, response)
			} else {
				details = te.buildLoggingAlertMessage(rule, response)
			}
		case "auditing":
			details = te.buildAuditingAlertMessage(rule, response)
		default:
			details = te.buildDefaultAlertMessage(rule, response)
		}
		// 合并（自定义在上，详情在下）
		if custom == "" {
			return details
		}
		return custom + "\n\n" + details
	}

	// 未设置自定义模板时，走系统默认详情
	// 根据索引类型确定事件类型
	// eventType 已在上面计算

	switch eventType {
	case "events":
		return te.buildEventAlertMessage(rule, response)
	case "logging":
		// 根据规则名称选择不同的日志模板
		if strings.Contains(rule.Name, "系统组件") {
			return te.buildSystemComponentLoggingAlertMessage(rule, response)
		}
		return te.buildLoggingAlertMessage(rule, response)
	case "auditing":
		return te.buildAuditingAlertMessage(rule, response)
	default:
		return te.buildDefaultAlertMessage(rule, response)
	}
}

// buildCustomAlertMessage 使用 AlertText/AlertTextArgs 构建自定义告警文本
func (te *TemplateEngine) buildCustomAlertMessage(rule types.AlertRule, response *types.OpenSearchResponse) string {
	text := rule.AlertText
	var source map[string]interface{}
	if len(response.Hits.Hits) > 0 {
		source = response.Hits.Hits[0].Source
	} else {
		source = make(map[string]interface{})
	}

	// 占位符替换：支持 ${path.to.field}
	placeholder := regexp.MustCompile(`\$\{([^}]+)\}`)
	text = placeholder.ReplaceAllStringFunc(text, func(m string) string {
		sub := placeholder.FindStringSubmatch(m)
		if len(sub) < 2 {
			return ""
		}
		val := te.getValueByPath(source, strings.TrimSpace(sub[1]))
		return val
	})

	if len(rule.AlertTextArgs) == 0 {
		return text
	}

	// 追加显示指定字段，便于上下文信息呈现
	var b strings.Builder
	b.WriteString(text)
	b.WriteString("\n\n")
	b.WriteString("数据字段:\n")
	for _, path := range rule.AlertTextArgs {
		p := strings.TrimSpace(path)
		if p == "" {
			continue
		}
		v := te.getValueByPath(source, p)
		b.WriteString(fmt.Sprintf("- %s: %s\n", p, v))
	}
	return b.String()
}

// detectEventType 检测事件类型
func (te *TemplateEngine) detectEventType(index string) string {
	if strings.Contains(index, "events") {
		return "events"
	} else if strings.Contains(index, "logging") {
		return "logging"
	} else if strings.Contains(index, "auditing") {
		return "auditing"
	}
	return "default"
}

// buildEventAlertMessage 构建事件告警消息
func (te *TemplateEngine) buildEventAlertMessage(rule types.AlertRule, response *types.OpenSearchResponse) string {
	if len(response.Hits.Hits) == 0 {
		return fmt.Sprintf("规则 %s 触发告警，匹配 %d 条事件记录", rule.Name, response.Hits.Total.Value)
	}

	// 取第一条记录作为示例
	hit := response.Hits.Hits[0].Source

	// 提取事件信息
	reason := te.getStringValue(hit, "reason")
	message := te.getStringValue(hit, "message")
	eventType := te.getStringValue(hit, "type")

	// 提取对象信息
	involvedObject := te.getMapValue(hit, "involvedObject")
	objectKind := te.getStringValue(involvedObject, "kind")
	objectName := te.getStringValue(involvedObject, "name")
	objectNamespace := te.getStringValue(involvedObject, "namespace")

	// 提取时间信息
	firstTimestamp := te.getTimeValue(hit, "firstTimestamp")
	lastTimestamp := te.getTimeValue(hit, "lastTimestamp")
	count := te.getIntValue(hit, "count")

	return fmt.Sprintf(`🚨 **Kubernetes 事件告警**

**规则名称:** %s
**事件类型:** %s
**事件原因:** %s
**资源类型:** %s
**资源名称:** %s
**命名空间:** %s
**事件消息:** %s
**首次发生:** %s
**最后发生:** %s
**发生次数:** %d
**匹配记录数:** %d`,
		rule.Name, eventType, reason, objectKind, objectName, objectNamespace,
		message, firstTimestamp, lastTimestamp, count, response.Hits.Total.Value)
}

// buildLoggingAlertMessage 构建日志告警消息
func (te *TemplateEngine) buildLoggingAlertMessage(rule types.AlertRule, response *types.OpenSearchResponse) string {
	if len(response.Hits.Hits) == 0 {
		return fmt.Sprintf("规则 %s 触发告警，匹配 %d 条日志记录", rule.Name, response.Hits.Total.Value)
	}

	// 取第一条记录作为示例
	hit := response.Hits.Hits[0].Source

	// 提取日志信息
	log := te.getStringValue(hit, "log")
	timestamp := te.getTimeValue(hit, "@timestamp")

	// 提取 Kubernetes 信息
	kubernetes := te.getMapValue(hit, "kubernetes")
	podName := te.getStringValue(kubernetes, "pod_name")
	namespace := te.getStringValue(kubernetes, "namespace_name")
	containerName := te.getStringValue(kubernetes, "container_name")
	containerImage := te.getStringValue(kubernetes, "container_image")

	// 截取日志内容（避免过长）
	if len(log) > 500 {
		log = log[:500] + "..."
	}

	// 根据规则名称确定告警类型
	alertType := "应用日志告警"
	if strings.Contains(rule.Name, "系统组件") {
		alertType = "系统组件日志告警"
	} else if strings.Contains(rule.Name, "Pod") {
		alertType = "Pod日志告警"
	}

	// 构建基础信息
	baseInfo := fmt.Sprintf("🚨 **%s**\n\n"+
		"**时间窗口:** 最近%d分钟\n"+
		"**阈值:** %d条\n"+
		"**实际匹配:** %d条",
		alertType, rule.Timeframe/60, rule.Threshold, response.Hits.Total.Value)

	// 构建Pod信息（如果存在）
	podInfo := ""
	if podName != "" {
		// 根据命名空间显示不同的标签
		namespaceLabel := "命名空间"
		if namespace == "kube-system" {
			namespaceLabel = "系统命名空间"
		} else if namespace == "default" {
			namespaceLabel = "默认命名空间"
		} else if namespace == "kubesphere-system" {
			namespaceLabel = "KubeSphere系统命名空间"
		}

		podInfo = fmt.Sprintf("\n\n**Pod 名称:** %s\n"+
			"**%s:** %s\n"+
			"**容器名称:** %s",
			podName, namespaceLabel, namespace, containerName)

		if containerImage != "" {
			podInfo += fmt.Sprintf("\n**容器镜像:** %s", containerImage)
		}
	}

	// 构建日志信息
	logInfo := fmt.Sprintf("\n**日志时间:** %s\n"+
		"**错误日志:** \n"+
		"```\n"+
		"%s\n"+
		"```\n"+
		"以上仅为1条示例日志，实际匹配了%d条错误日志",
		timestamp, log, response.Hits.Total.Value)

	return baseInfo + podInfo + logInfo
}

// buildSystemComponentLoggingAlertMessage 构建系统组件日志告警消息
func (te *TemplateEngine) buildSystemComponentLoggingAlertMessage(rule types.AlertRule, response *types.OpenSearchResponse) string {
	if len(response.Hits.Hits) == 0 {
		return fmt.Sprintf("规则 %s 触发告警，匹配 %d 条系统组件日志记录", rule.Name, response.Hits.Total.Value)
	}

	// 取第一条记录作为示例
	hit := response.Hits.Hits[0].Source

	// 提取日志信息
	log := te.getStringValue(hit, "log")
	timestamp := te.getTimeValue(hit, "@timestamp")

	// 提取 Kubernetes 信息
	kubernetes := te.getMapValue(hit, "kubernetes")
	podName := te.getStringValue(kubernetes, "pod_name")
	namespace := te.getStringValue(kubernetes, "namespace_name")
	containerName := te.getStringValue(kubernetes, "container_name")
	containerImage := te.getStringValue(kubernetes, "container_image")

	// 截取日志内容（避免过长）
	if len(log) > 500 {
		log = log[:500] + "..."
	}

	// 构建基础信息
	baseInfo := fmt.Sprintf("🚨 **系统组件日志告警**\n\n"+
		"**时间窗口:** 最近%d分钟\n"+
		"**阈值:** %d条\n"+
		"**实际匹配:** %d条",
		rule.Timeframe/60, rule.Threshold, response.Hits.Total.Value)

	// 构建系统组件信息
	componentInfo := ""
	if podName != "" {
		// 根据组件类型显示不同的标签
		componentLabel := "组件名称"
		if containerName == "kubelet" {
			componentLabel = "Kubelet组件"
		} else if containerName == "dockerd" {
			componentLabel = "Docker守护进程"
		} else if containerName == "kube-apiserver" {
			componentLabel = "API服务器"
		} else if containerName == "kube-controller-manager" {
			componentLabel = "控制器管理器"
		} else if containerName == "kube-scheduler" {
			componentLabel = "调度器"
		} else if containerName == "coredns" {
			componentLabel = "DNS服务"
		} else if containerName == "etcd" {
			componentLabel = "etcd存储"
		}

		componentInfo = fmt.Sprintf("\n\n**节点名称:** %s\n"+
			"**命名空间:** %s\n"+
			"**%s:** %s",
			podName, namespace, componentLabel, containerName)

		if containerImage != "" {
			componentInfo += fmt.Sprintf("\n**组件镜像:** %s", containerImage)
		}
	}

	// 构建日志信息
	logInfo := fmt.Sprintf("\n**日志时间:** %s\n"+
		"**错误日志:** \n"+
		"```\n"+
		"%s\n"+
		"```\n"+
		"以上仅为1条示例日志，实际匹配了%d条系统组件错误日志",
		timestamp, log, response.Hits.Total.Value)

	return baseInfo + componentInfo + logInfo
}

// buildAuditingAlertMessage 构建审计告警消息
func (te *TemplateEngine) buildAuditingAlertMessage(rule types.AlertRule, response *types.OpenSearchResponse) string {
	if len(response.Hits.Hits) == 0 {
		return fmt.Sprintf("规则 %s 触发告警，匹配 %d 条审计记录", rule.Name, response.Hits.Total.Value)
	}

	// 取第一条记录作为示例
	hit := response.Hits.Hits[0].Source

	// 提取审计信息
	level := te.getStringValue(hit, "Level")
	message := te.getStringValue(hit, "Message")
	verb := te.getStringValue(hit, "Verb")
	timestamp := te.getTimeValue(hit, "@timestamp")

	// 提取对象信息
	objectRef := te.getMapValue(hit, "ObjectRef")
	resource := te.getStringValue(objectRef, "Resource")
	objectName := te.getStringValue(objectRef, "Name")
	objectNamespace := te.getStringValue(objectRef, "Namespace")

	// 提取用户信息
	user := te.getMapValue(hit, "User")
	username := te.getStringValue(user, "Username")
	userUID := te.getStringValue(user, "UID")

	// 提取响应信息
	responseStatus := te.getMapValue(hit, "ResponseStatus")
	statusCode := te.getIntValue(responseStatus, "code")

	return fmt.Sprintf(`🚨 **安全审计告警**

**规则名称:** %s
**审计级别:** %s
**操作类型:** %s
**资源类型:** %s
**资源名称:** %s
**命名空间:** %s
**操作用户:** %s (UID: %s)
**响应状态:** %d
**审计消息:** %s
**操作时间:** %s
**匹配记录数:** %d`,
		rule.Name, level, verb, resource, objectName, objectNamespace,
		username, userUID, statusCode, message, timestamp, response.Hits.Total.Value)
}

// buildDefaultAlertMessage 构建默认告警消息
func (te *TemplateEngine) buildDefaultAlertMessage(rule types.AlertRule, response *types.OpenSearchResponse) string {
	return fmt.Sprintf(`🚨 **OpenSearch 告警**

**规则名称:** %s
**匹配记录数:** %d
**告警时间:** %s
**索引模式:** %s`,
		rule.Name, response.Hits.Total.Value,
		time.Now().Format("2006-01-02 15:04:05"), rule.Index)
}

// 辅助方法
func (te *TemplateEngine) getStringValue(data map[string]interface{}, key string) string {
	if val, ok := data[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// getValueByPath 从嵌套 map 中按点路径获取值，并返回字符串
func (te *TemplateEngine) getValueByPath(root map[string]interface{}, path string) string {
	if path == "" {
		return ""
	}
	parts := strings.Split(path, ".")
	var cur interface{} = root
	for _, part := range parts {
		if m, ok := cur.(map[string]interface{}); ok {
			cur = m[part]
		} else {
			return ""
		}
	}
	switch v := cur.(type) {
	case string:
		return v
	case float64:
		return fmt.Sprintf("%v", v)
	case int:
		return fmt.Sprintf("%d", v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	case nil:
		return ""
	default:
		// 尝试序列化为 JSON
		if b, err := json.Marshal(v); err == nil {
			return string(b)
		}
		return ""
	}
}

func (te *TemplateEngine) getIntValue(data map[string]interface{}, key string) int {
	if val, ok := data[key]; ok {
		if intVal, ok := val.(int); ok {
			return intVal
		}
		if floatVal, ok := val.(float64); ok {
			return int(floatVal)
		}
	}
	return 0
}

func (te *TemplateEngine) getMapValue(data map[string]interface{}, key string) map[string]interface{} {
	if val, ok := data[key]; ok {
		if mapVal, ok := val.(map[string]interface{}); ok {
			return mapVal
		}
	}
	return make(map[string]interface{})
}

func (te *TemplateEngine) getTimeValue(data map[string]interface{}, key string) string {
	if val, ok := data[key]; ok {
		if str, ok := val.(string); ok {
			// 尝试解析时间格式
			if t, err := time.Parse(time.RFC3339, str); err == nil {
				// 转换为本地时间（CST，UTC+8）
				localTime := t.In(time.FixedZone("CST", 8*60*60))
				return localTime.Format("2006-01-02 15:04:05")
			}
			return str
		}
	}
	return ""
}

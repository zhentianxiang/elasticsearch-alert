package alert

import (
	"context"
	"fmt"
	"opensearch-alert/internal/database"
	"opensearch-alert/internal/notification"
	"opensearch-alert/internal/opensearch"
	"opensearch-alert/pkg/types"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
)

// Engine 告警引擎
type Engine struct {
	config           *types.Config
	opensearchClient *opensearch.Client
	notifier         *notification.Notifier
	database         *database.Database
	templateEngine   *TemplateEngine
	rules            []types.AlertRule
	alertStatuses    map[string]*types.AlertStatus
	statusMutex      sync.RWMutex
	logger           *logrus.Logger
	cron             *cron.Cron
}

// NewEngine 创建新的告警引擎
func NewEngine(config *types.Config, opensearchClient *opensearch.Client, notifier *notification.Notifier, database *database.Database, logger *logrus.Logger) *Engine {
	return &Engine{
		config:           config,
		opensearchClient: opensearchClient,
		notifier:         notifier,
		database:         database,
		templateEngine:   NewTemplateEngine(),
		alertStatuses:    make(map[string]*types.AlertStatus),
		logger:           logger,
		cron:             cron.New(cron.WithSeconds()),
	}
}

// LoadRules 加载告警规则
func (e *Engine) LoadRules(rules []types.AlertRule) {
	e.rules = rules
	e.logger.Infof("加载了 %d 个告警规则", len(rules))
}

// Start 启动告警引擎
func (e *Engine) Start() error {
	// 添加定时任务
	_, err := e.cron.AddFunc(fmt.Sprintf("@every %ds", e.config.AlertEngine.RunInterval), e.runRules)
	if err != nil {
		return fmt.Errorf("添加定时任务失败: %w", err)
	}

	e.cron.Start()
	e.logger.Info("告警引擎已启动")
	return nil
}

// Stop 停止告警引擎
func (e *Engine) Stop() {
	e.cron.Stop()
	e.logger.Info("告警引擎已停止")
}

// runRules 运行所有规则
func (e *Engine) runRules() {
	e.logger.Debug("开始执行告警规则检查")

	for _, rule := range e.rules {
		go e.runRule(rule)
	}
}

// runRule 运行单个规则
func (e *Engine) runRule(rule types.AlertRule) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	e.logger.Debugf("执行规则: %s", rule.Name)

	// 多副本互斥：获取规则级租约锁
	instanceID := getInstanceID()
	ttl := 30 // 默认租约30秒
	locked, err := e.database.AcquireRuleLock(rule.Name, instanceID, ttl)
	if err != nil {
		e.logger.Warnf("获取规则锁失败 %s: %v", rule.Name, err)
		return
	}
	if !locked {
		e.logger.Debugf("规则 %s 未获得锁，跳过本轮", rule.Name)
		return
	}
	defer func() {
		if err := e.database.ReleaseRuleLock(rule.Name, instanceID); err != nil {
			e.logger.Warnf("释放规则锁失败 %s: %v", rule.Name, err)
		}
	}()

	// 检查告警抑制
	if e.isSuppressed(rule.Name) {
		e.logger.Debugf("规则 %s 被抑制", rule.Name)
		return
	}

	// 构建查询
	query := e.opensearchClient.BuildTimeRangeQuery(rule, e.config.AlertEngine.BufferTime)

	// 执行查询
	response, err := e.opensearchClient.Search(ctx, rule.Index, query)
	if err != nil {
		e.logger.Errorf("规则 %s 查询失败: %v", rule.Name, err)
		return
	}

	// 检查是否触发告警
	if e.shouldTriggerAlert(rule, response) {
		e.triggerAlert(rule, response)
	}
}

// getInstanceID 返回实例标识，用于分布式锁标记
func getInstanceID() string {
	if v := os.Getenv("INSTANCE_ID"); v != "" {
		return v
	}
	h, _ := os.Hostname()
	return h
}

// shouldTriggerAlert 检查是否应该触发告警
func (e *Engine) shouldTriggerAlert(rule types.AlertRule, response *types.OpenSearchResponse) bool {
	count := response.Hits.Total.Value

	switch rule.Type {
	case "frequency":
		return count >= rule.Threshold
	case "any":
		return count > 0
	case "spike":
		// 这里可以实现流量突增检测逻辑
		return count >= rule.Threshold
	case "flatline":
		// 这里可以实现流量低于阈值检测逻辑
		return count < rule.Threshold
	case "change":
		// 这里可以实现字段值变化检测逻辑
		return count > 0
	default:
		return count >= rule.Threshold
	}
}

// triggerAlert 触发告警
func (e *Engine) triggerAlert(rule types.AlertRule, response *types.OpenSearchResponse) {
	e.logger.Infof("规则 %s 触发告警，匹配 %d 条记录", rule.Name, response.Hits.Total.Value)

	// 创建告警
	alert := &types.Alert{
		ID:        fmt.Sprintf("%s-%d", rule.Name, time.Now().Unix()),
		RuleName:  rule.Name,
		Level:     e.determineAlertLevel(rule, response), // 根据规则和内容确定级别
		Message:   e.buildAlertMessage(rule, response),
		Timestamp: time.Now(),
		Data:      e.extractAlertData(response),
		Count:     response.Hits.Total.Value,
		Matches:   len(response.Hits.Hits),
	}

	// 去重：在发送与落库前检查
	dedupeTTL := 120 // 秒（可后续做成配置）
	shouldSend, err := e.database.ShouldSendAndTouch(alert.RuleName, alert.Level, alert.Message, dedupeTTL)
	if err != nil {
		e.logger.Warnf("去重检查失败（忽略错误继续）: %v", err)
	}
	if !shouldSend {
		e.logger.Infof("规则 %s 去重命中，跳过发送与落库", rule.Name)
		return
	}

	// 发送通知
	if err := e.notifier.SendAlert(alert); err != nil {
		e.logger.Errorf("发送告警通知失败: %v", err)
	}

	// 保存告警到数据库
	if err := e.database.SaveAlert(alert); err != nil {
		e.logger.Errorf("保存告警到数据库失败: %v", err)
	}

	// 更新告警状态
	e.updateAlertStatus(rule.Name, alert)

	// 记录告警到 OpenSearch
	e.recordAlert(alert)
}

// determineAlertLevel 根据规则和内容确定告警级别
func (e *Engine) determineAlertLevel(rule types.AlertRule, response *types.OpenSearchResponse) string {
	// 优先使用规则中定义的级别
	if rule.Level != "" {
		e.logger.Debugf("使用规则定义级别: %s -> %s", rule.Name, rule.Level)
		return rule.Level
	}

	// 如果没有定义级别，则根据规则名称自动判断
	ruleName := strings.ToLower(rule.Name)

	// 严重告警：系统组件错误、安全事件、FATAL错误
	if strings.Contains(ruleName, "系统组件") && strings.Contains(ruleName, "错误") {
		e.logger.Debugf("自动判断级别: %s -> Critical (系统组件错误)", rule.Name)
		return "Critical"
	}
	if strings.Contains(ruleName, "安全") {
		e.logger.Debugf("自动判断级别: %s -> Critical (安全事件)", rule.Name)
		return "Critical"
	}
	if strings.Contains(ruleName, "fatal") || strings.Contains(ruleName, "panic") {
		e.logger.Debugf("自动判断级别: %s -> Critical (FATAL/PANIC)", rule.Name)
		return "Critical"
	}

	// 高优先级告警：应用错误日志、大量系统组件警告
	if strings.Contains(ruleName, "错误") && !strings.Contains(ruleName, "系统组件") {
		e.logger.Debugf("自动判断级别: %s -> High (应用错误)", rule.Name)
		return "High"
	}
	if strings.Contains(ruleName, "系统组件") && strings.Contains(ruleName, "警告") {
		e.logger.Debugf("自动判断级别: %s -> High (系统组件警告)", rule.Name)
		return "High"
	}

	// 中等优先级告警：警告日志
	if strings.Contains(ruleName, "警告") && !strings.Contains(ruleName, "系统组件") {
		e.logger.Debugf("自动判断级别: %s -> Medium (应用警告)", rule.Name)
		return "Medium"
	}

	// 低优先级告警：其他情况
	e.logger.Debugf("自动判断级别: %s -> Low (默认)", rule.Name)
	return "Low"
}

// buildAlertMessage 构建告警消息
func (e *Engine) buildAlertMessage(rule types.AlertRule, response *types.OpenSearchResponse) string {
	// 使用模板引擎构建消息
	return e.templateEngine.BuildAlertMessage(rule, response)
}

// extractAlertData 提取告警数据
func (e *Engine) extractAlertData(response *types.OpenSearchResponse) map[string]interface{} {
	data := make(map[string]interface{})

	if len(response.Hits.Hits) > 0 {
		// 取第一条记录作为示例数据
		data["sample_hit"] = response.Hits.Hits[0].Source
	}

	data["total_hits"] = response.Hits.Total.Value
	data["max_score"] = response.Hits.MaxScore

	return data
}

// updateAlertStatus 更新告警状态
func (e *Engine) updateAlertStatus(ruleName string, alert *types.Alert) {
	e.statusMutex.Lock()
	defer e.statusMutex.Unlock()

	status := e.alertStatuses[ruleName]
	if status == nil {
		status = &types.AlertStatus{
			RuleName: ruleName,
		}
		e.alertStatuses[ruleName] = status
	}

	status.LastAlert = alert.Timestamp
	status.AlertCount++

	// 设置抑制时间
	if e.config.AlertSuppression.Enabled {
		suppressDuration := time.Duration(e.config.AlertSuppression.RealertMinutes) * time.Minute

		// 指数级抑制
		if e.config.AlertSuppression.ExponentialRealert.Enabled {
			exponentialHours := e.config.AlertSuppression.ExponentialRealert.Hours
			suppressDuration = time.Duration(exponentialHours) * time.Hour * time.Duration(status.AlertCount)
		}

		status.Suppressed = true
		status.SuppressUntil = time.Now().Add(suppressDuration)
	}
}

// isSuppressed 检查规则是否被抑制
func (e *Engine) isSuppressed(ruleName string) bool {
	e.statusMutex.RLock()
	defer e.statusMutex.RUnlock()

	status := e.alertStatuses[ruleName]
	if status == nil {
		e.logger.Debugf("规则 %s 没有告警状态记录", ruleName)
		return false
	}

	if !status.Suppressed {
		e.logger.Debugf("规则 %s 未被抑制", ruleName)
		return false
	}

	// 检查抑制时间是否已过
	if time.Now().After(status.SuppressUntil) {
		e.logger.Infof("规则 %s 抑制时间已过，解除抑制", ruleName)
		status.Suppressed = false
		return false
	}

	e.logger.Debugf("规则 %s 被抑制，抑制到 %s", ruleName, status.SuppressUntil.Format("2006-01-02 15:04:05"))
	return true
}

// recordAlert 记录告警到 OpenSearch
func (e *Engine) recordAlert(alert *types.Alert) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	index := e.config.AlertEngine.WritebackIndex
	err := e.opensearchClient.Index(ctx, index, alert.ID, alert)
	if err != nil {
		e.logger.Errorf("记录告警到 OpenSearch 失败: %v", err)
	}
}

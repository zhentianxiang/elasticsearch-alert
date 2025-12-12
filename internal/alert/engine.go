package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/robfig/cron/v3"

	"elasticsearch-alert/internal/config"
	eswrap "elasticsearch-alert/internal/elasticsearch"
	"elasticsearch-alert/internal/notification"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Engine struct {
	cfg       *config.Config
	es        *eswrap.Client
	notifiers []notification.Notifier

	cron     *cron.Cron
	location *time.Location

	rules        []Rule
	lastAlertAt  map[string]time.Time
	defaultQuiet time.Duration
	sampleSize   int
}

func NewEngine(cfg *config.Config, es *eswrap.Client, notifiers []notification.Notifier) (*Engine, error) {
	loc, err := time.LoadLocation(cfg.Scheduler.Timezone)
	if err != nil {
		loc = time.Local
	}
	c := cron.New(cron.WithLocation(loc), cron.WithSeconds())

	engine := &Engine{
		cfg:          cfg,
		es:           es,
		notifiers:    notifiers,
		cron:         c,
		location:     loc,
		lastAlertAt:  make(map[string]time.Time),
		defaultQuiet: cfg.Rules.GetDefaultQuietPeriod(),
		sampleSize:   cfg.Rules.SampleSize,
	}
	if err := engine.loadRules(cfg.Rules.Directory); err != nil {
		return nil, err
	}
	return engine, nil
}

func (e *Engine) Start() error {
	for i := range e.rules {
		r := e.rules[i]
		_, err := e.cron.AddFunc(r.Cron, func() { e.executeRule(r) })
		if err != nil {
			return fmt.Errorf("add cron for rule %q: %w", r.Name, err)
		}
		log.Printf("rule registered: %s cron=%s window=%s", r.Name, r.Cron, r.TimeWindow)
	}
	e.cron.Start()
	return nil
}

func (e *Engine) Stop() {
	ctx := e.cron.Stop()
	<-ctx.Done()
}

func (e *Engine) loadRules(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read rules dir: %w", err)
	}
	var rules []Rule
	for _, entry := range entries {
		if entry.IsDir() || !(strings.HasSuffix(entry.Name(), ".yaml") || strings.HasSuffix(entry.Name(), ".yml")) {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read rule %s: %w", path, err)
		}
		var r Rule
		if err := yaml.Unmarshal(data, &r); err != nil {
			return fmt.Errorf("unmarshal rule %s: %w", path, err)
		}
		if r.Name == "" || r.Index == "" || r.Cron == "" || r.TimeWindow == "" {
			return fmt.Errorf("invalid rule %s: name/index/cron/timeWindow required", path)
		}
		rules = append(rules, r)
	}
	e.rules = rules
	return nil
}

func (e *Engine) executeRule(r Rule) {
	defer func() {
		if rec := recover(); rec != nil {
			log.Printf("panic in rule %s: %v", r.Name, rec)
		}
	}()
	now := time.Now().In(e.location)
	if !e.shouldFire(r, now) {
		return
	}
	count, samples, err := e.queryCountAndSamples(r)
	if err != nil {
		log.Printf("rule %s query error: %v", r.Name, err)
		return
	}
	if !e.hitThreshold(r, count) {
		return
	}
	title := fmt.Sprintf("[Elasticsearch Alert] %s", r.Name)
	body := e.renderBody(r, count, samples)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, ch := range r.Alerts.Channels {
		for _, n := range e.notifiers {
			if n.Name() == ch || (ch == "console" && n.Name() == "console") {
				if err := n.Send(ctx, title, body); err != nil {
					log.Printf("send alert via %s error: %v", n.Name(), err)
				}
			}
		}
	}
	e.lastAlertAt[r.Name] = now
}

func (e *Engine) shouldFire(r Rule, now time.Time) bool {
	last, ok := e.lastAlertAt[r.Name]
	if !ok {
		return true
	}
	quiet := r.Dedup.GetQuietPeriod(e.defaultQuiet)
	return now.Sub(last) >= quiet
}

func (e *Engine) hitThreshold(r Rule, count int) bool {
	if r.Threshold.CountGt != nil {
		return count > *r.Threshold.CountGt
	}
	return false
}

func (e *Engine) renderBody(r Rule, count int, samples []map[string]any) string {
	now := time.Now().In(e.location)
	severity := r.Severity
	if severity == "" {
		severity = "Medium"
	}

	var b strings.Builder
	// 标题行（正文内部的视觉标题，各渠道外层也有标题）
	b.WriteString("🚨 **Elasticsearch 日志告警**\n\n")

	if r.Description != "" {
		b.WriteString(r.Description + "\n\n")
	}

	// 概览信息
	b.WriteString("📊 **告警概览**\n")
	b.WriteString(fmt.Sprintf("- **规则名称：** %s\n", r.Name))
	b.WriteString(fmt.Sprintf("- **告警级别：** %s\n", severity))
	b.WriteString(fmt.Sprintf("- **触发时间：** %s\n", now.Format("2006-01-02 15:04:05")))
	b.WriteString(fmt.Sprintf("- **索引：** %s\n", r.Index))
	b.WriteString(fmt.Sprintf("- **时间窗：** %s\n", r.TimeWindow))
	b.WriteString(fmt.Sprintf("- **命中条数：** %d\n", count))
	if r.Threshold.CountGt != nil {
		b.WriteString(fmt.Sprintf("- **阈值：** > %d 条\n", *r.Threshold.CountGt))
	}
	if r.QueryString != "" {
		b.WriteString(fmt.Sprintf("- **查询：** %s\n", r.QueryString))
	} else if r.DSL != nil {
		b.WriteString("- **查询：** DSL\n")
	}

	// 只展示一条代表性的样例，突出节点/Pod/镜像/错误日志
	if len(samples) > 0 {
		doc := samples[0]
		ts, _ := doc["@timestamp"].(string)
		indexName, _ := doc["_index"].(string)
		docID, _ := doc["_id"].(string)
		node, _ := doc["kubernetes_host"].(string)
		ns, _ := doc["kubernetes_namespace_name"].(string)
		pod, _ := doc["kubernetes_pod_name"].(string)
		image, _ := doc["kubernetes_container_image"].(string)
		msg, _ := doc["message"].(string)
		truncated := false
		if len(msg) > 800 {
			truncated = true
			msg = msg[:800] + "..."
		}

		b.WriteString("\n📌 **本次告警目标**\n")
		if node != "" {
			b.WriteString(fmt.Sprintf("- **节点名称：** %s\n", node))
		}
		if ns != "" {
			b.WriteString(fmt.Sprintf("- **命名空间：** %s\n", ns))
		}
		if pod != "" {
			b.WriteString(fmt.Sprintf("- **Pod 名称：** %s\n", pod))
		}
		if image != "" {
			b.WriteString(fmt.Sprintf("- **Pod 镜像：** %s\n", image))
		}
		if ts != "" {
			b.WriteString(fmt.Sprintf("- **日志时间：** %s\n", ts))
		}

		if msg != "" {
			b.WriteString("\n🧾 **错误日志**\n")
			b.WriteString(msg)
			if truncated {
				b.WriteString("\n...(日志内容较长，已截断显示)")
			}
			b.WriteString("\n")
		}

		// 详细日志链接：指向 Elasticsearch 中该条文档的 _doc API，仅查看本次命中的这一条日志
		if indexName != "" && docID != "" && len(e.cfg.Elasticsearch.Addresses) > 0 {
			base := e.cfg.Elasticsearch.Addresses[0]
			base = strings.TrimRight(base, "/")
			detailURL := fmt.Sprintf("%s/%s/_doc/%s?pretty", base, indexName, docID)
			b.WriteString("\n🔗 **详细日志链接：** ")
			b.WriteString(detailURL)
			b.WriteString("\n")
		}
	}

	// 方便在通知模版底部额外追加 @所有人，这里不直接处理 @ 文本
	return b.String()
}

func (e *Engine) queryCountAndSamples(r Rule) (int, []map[string]any, error) {
	window := r.TimeWindow
	if window == "" {
		window = "5m"
	}
	rangeGte := fmt.Sprintf("now-%s", window)
	rangeLt := "now"

	query := map[string]any{
		"size": e.sampleSize,
		"sort": []map[string]any{
			{"@timestamp": map[string]any{"order": "desc"}},
		},
		"track_total_hits": true,
		"query": map[string]any{
			"bool": map[string]any{
				"filter": []any{
					map[string]any{
						"range": map[string]any{
							"@timestamp": map[string]any{
								"gte": rangeGte,
								"lt":  rangeLt,
							},
						},
					},
				},
			},
		},
	}
	boolQuery := query["query"].(map[string]any)["bool"].(map[string]any)
	filters := boolQuery["filter"].([]any)
	if r.QueryString != "" {
		filters = append(filters, map[string]any{
			"query_string": map[string]any{
				"query":            r.QueryString,
				"default_operator": "AND",
			},
		})
	} else if r.DSL != nil {
		filters = append(filters, r.DSL)
	}
	boolQuery["filter"] = filters

	var buf bytes.Buffer
	_ = json.NewEncoder(&buf).Encode(query)

	res, err := e.es.Search(r.Index, &buf)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	if res.IsError() {
		return 0, nil, fmt.Errorf("search error: %s", res.String())
	}
	var parsed struct {
		Hits struct {
			Total struct {
				Value int `json:"value"`
			} `json:"total"`
			Hits []struct {
				Index  string         `json:"_index"`
				ID     string         `json:"_id"`
				Source map[string]any `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(res.Body).Decode(&parsed); err != nil {
		return 0, nil, err
	}
	samples := make([]map[string]any, 0, len(parsed.Hits.Hits))
	for _, h := range parsed.Hits.Hits {
		doc := h.Source
		if doc == nil {
			doc = make(map[string]any)
		}
		// 将 _index 与 _id 一并放入样例文档中，便于后续生成详细日志链接
		doc["_index"] = h.Index
		doc["_id"] = h.ID
		samples = append(samples, doc)
	}
	return parsed.Hits.Total.Value, samples, nil
}

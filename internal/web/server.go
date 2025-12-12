package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"

	"elasticsearch-alert/internal/config"
	eswrap "elasticsearch-alert/internal/elasticsearch"
	"elasticsearch-alert/internal/logging"
)

// Server 提供一个简单的只读 Web 页面，用于查看告警命中的单条日志详情。
type Server struct {
	cfg *config.Config
	es  *eswrap.Client
}

func NewServer(cfg *config.Config, es *eswrap.Client) *Server {
	return &Server{
		cfg: cfg,
		es:  es,
	}
}

// Start 会在配置的监听地址上启动 HTTP 服务（阻塞调用）。
// 建议在单独的 goroutine 中启动。
func (s *Server) Start() error {
	if !s.cfg.Web.Enabled {
		logging.Infof("Web 服务未开启（配置中 web.enabled=false）")
		return nil
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/logs", s.handleLogDetail)

	addr := s.cfg.Web.Listen
	if addr == "" {
		addr = ":8080"
	}
	logging.Infof("Web 服务已启动，监听地址=%s", addr)
	return http.ListenAndServe(addr, mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) handleLogDetail(w http.ResponseWriter, r *http.Request) {
	index := r.URL.Query().Get("index")
	id := r.URL.Query().Get("id")
	if index == "" || id == "" {
		http.Error(w, "缺少 index 或 id 参数", http.StatusBadRequest)
		return
	}

	res, err := s.es.Get(index, id)
	if err != nil {
		http.Error(w, fmt.Sprintf("查询日志失败: %v", err), http.StatusInternalServerError)
		return
	}
	defer res.Body.Close()
	if res.IsError() {
		http.Error(w, fmt.Sprintf("Elasticsearch 返回错误: %s", res.String()), http.StatusInternalServerError)
		return
	}
	var doc struct {
		Index  string                 `json:"_index"`
		ID     string                 `json:"_id"`
		Source map[string]interface{} `json:"_source"`
	}
	if err := json.NewDecoder(res.Body).Decode(&doc); err != nil {
		http.Error(w, fmt.Sprintf("解析 Elasticsearch 响应失败: %v", err), http.StatusInternalServerError)
		return
	}

	pretty, _ := json.MarshalIndent(doc.Source, "", "  ")

	data := struct {
		Index     string
		ID        string
		Pretty    string
		Raw       map[string]interface{}
		Title     string
		Timestamp string
		Namespace string
		Pod       string
		Node      string
		Image     string
		Message   string
	}{
		Index:  doc.Index,
		ID:     doc.ID,
		Pretty: string(pretty),
		Raw:    doc.Source,
		Title:  "Elasticsearch 日志告警详情",
	}

	if ts, ok := doc.Source["@timestamp"].(string); ok {
		data.Timestamp = ts
	}
	if ns, ok := doc.Source["kubernetes_namespace_name"].(string); ok {
		data.Namespace = ns
	}
	if pod, ok := doc.Source["kubernetes_pod_name"].(string); ok {
		data.Pod = pod
	}
	if node, ok := doc.Source["kubernetes_host"].(string); ok {
		data.Node = node
	}
	if image, ok := doc.Source["kubernetes_container_image"].(string); ok {
		data.Image = image
	}
	if msg, ok := doc.Source["message"].(string); ok {
		data.Message = msg
	}

	tmpl := template.Must(template.New("log-detail").Parse(logDetailHTML))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		logging.Errorf("渲染日志详情页面失败: %v", err)
	}
}

const logDetailHTML = `
<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <title>{{.Title}}</title>
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <style>
    body {
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
      margin: 0;
      padding: 0;
      background-color: #f5f5f7;
      color: #27272a;
    }
    .container {
      max-width: 960px;
      margin: 32px auto;
      padding: 0 16px;
    }
    .card {
      background: #ffffff;
      border-radius: 12px;
      box-shadow: 0 10px 30px rgba(15,23,42,0.08);
      overflow: hidden;
      border: 1px solid rgba(148,163,184,0.4);
    }
    .card-header {
      padding: 16px 20px;
      background: linear-gradient(90deg, #fee2e2, #f97316);
      display: flex;
      justify-content: space-between;
      align-items: center;
    }
    .card-header-title {
      font-size: 18px;
      font-weight: 600;
      color: #7f1d1d;
      display: flex;
      align-items: center;
      gap: 8px;
    }
    .badge {
      font-size: 12px;
      padding: 4px 10px;
      border-radius: 999px;
      background: rgba(248,250,252,0.9);
      color: #1f2933;
      border: 1px solid rgba(148,163,184,0.6);
    }
    .card-body {
      padding: 20px;
    }
    .section-title {
      font-size: 14px;
      font-weight: 600;
      color: #4b5563;
      margin-bottom: 8px;
      display: flex;
      align-items: center;
      gap: 6px;
      text-transform: uppercase;
      letter-spacing: 0.04em;
    }
    .meta-grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
      gap: 12px 16px;
      margin-bottom: 20px;
    }
    .meta-item-label {
      font-size: 12px;
      color: #6b7280;
      text-transform: uppercase;
      letter-spacing: 0.06em;
      margin-bottom: 2px;
    }
    .meta-item-value {
      font-size: 14px;
      color: #111827;
      word-break: break-all;
    }
    .message-box {
      background: #0b1120;
      color: #e5e7eb;
      border-radius: 8px;
      padding: 12px 14px;
      font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
      font-size: 13px;
      line-height: 1.5;
      white-space: pre-wrap;
      margin-bottom: 20px;
      border: 1px solid #1e293b;
    }
    .json-box {
      background: #f1f5f9;
      border-radius: 8px;
      padding: 12px 14px;
      font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
      font-size: 13px;
      line-height: 1.5;
      border: 1px solid #cbd5f5;
      overflow-x: auto;
      white-space: pre;
    }
    .footer {
      text-align: right;
      font-size: 12px;
      color: #9ca3af;
      padding-top: 8px;
    }
    @media (max-width: 640px) {
      .card {
        border-radius: 0;
      }
      .container {
        margin: 0;
        padding: 0;
      }
    }
  </style>
</head>
<body>
  <div class="container">
    <div class="card">
      <div class="card-header">
        <div class="card-header-title">
          <span>🚨</span>
          <span>{{.Title}}</span>
        </div>
        <div class="badge">单条日志详情</div>
      </div>
      <div class="card-body">
        <div class="section-title">
          <span>📍</span>
          <span>基本信息</span>
        </div>
        <div class="meta-grid">
          <div>
            <div class="meta-item-label">INDEX</div>
            <div class="meta-item-value">{{.Index}}</div>
          </div>
          <div>
            <div class="meta-item-label">DOCUMENT ID</div>
            <div class="meta-item-value">{{.ID}}</div>
          </div>
          {{if .Timestamp}}
          <div>
            <div class="meta-item-label">TIMESTAMP</div>
            <div class="meta-item-value">{{.Timestamp}}</div>
          </div>
          {{end}}
          {{if .Namespace}}
          <div>
            <div class="meta-item-label">NAMESPACE</div>
            <div class="meta-item-value">{{.Namespace}}</div>
          </div>
          {{end}}
          {{if .Pod}}
          <div>
            <div class="meta-item-label">POD</div>
            <div class="meta-item-value">{{.Pod}}</div>
          </div>
          {{end}}
          {{if .Node}}
          <div>
            <div class="meta-item-label">NODE</div>
            <div class="meta-item-value">{{.Node}}</div>
          </div>
          {{end}}
          {{if .Image}}
          <div>
            <div class="meta-item-label">IMAGE</div>
            <div class="meta-item-value">{{.Image}}</div>
          </div>
          {{end}}
        </div>

        {{if .Message}}
        <div class="section-title">
          <span>🧾</span>
          <span>日志内容</span>
        </div>
        <div class="message-box">{{.Message}}</div>
        {{end}}

        <div class="section-title">
          <span>🧩</span>
          <span>完整 JSON</span>
        </div>
        <div class="json-box">{{.Pretty}}</div>

        <div class="footer">
          Powered by Elasticsearch Alert
        </div>
      </div>
    </div>
  </div>
</body>
</html>
`

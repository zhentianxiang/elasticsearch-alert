package web

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"opensearch-alert/internal/alert"
	"opensearch-alert/internal/database"
	"opensearch-alert/internal/notification"
	"opensearch-alert/pkg/types"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// Server Web 服务器
type Server struct {
	config        *types.Config
	database      *database.Database
	notifier      *notification.Notifier
	engine        *alert.Engine
	logger        *logrus.Logger
	store         *sessions.CookieStore
	pageTemplates map[string]*template.Template
	router        *mux.Router
}

// NewServer 创建 Web 服务器
func NewServer(config *types.Config, database *database.Database, notifier *notification.Notifier, engine *alert.Engine, logger *logrus.Logger) *Server {
	// 注册User类型到gob编码器
	gob.Register(&types.User{})

	// 创建会话存储
	store := sessions.NewCookieStore([]byte(config.Web.SessionSecret))
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   int(config.Auth.SessionTimeout),
		HttpOnly: true,
		Secure:   false, // 在生产环境中应该设为 true
		SameSite: http.SameSiteLaxMode,
	}

	server := &Server{
		config:        config,
		database:      database,
		notifier:      notifier,
		engine:        engine,
		logger:        logger,
		store:         store,
		pageTemplates: make(map[string]*template.Template),
		router:        mux.NewRouter(),
	}

	// 加载模板
	server.loadTemplates()

	// 设置路由
	server.setupRoutes()

	return server
}

// loadTemplates 加载模板
func (s *Server) loadTemplates() {
	templatePath := s.config.Web.TemplatePath
	if templatePath == "" {
		templatePath = "web/templates"
	}
	s.pageTemplates = make(map[string]*template.Template)

	baseTmplPath := filepath.Join(templatePath, "base.html")

	pages, err := filepath.Glob(filepath.Join(templatePath, "*.html"))
	if err != nil {
		s.logger.Errorf("查找页面模板失败: %v", err)
		return
	}

	for _, page := range pages {
		name := filepath.Base(page)
		if name == "base.html" {
			continue
		}

		var tmpl *template.Template
		var parseErr error

		if name == "login.html" {
			tmpl, parseErr = template.ParseFiles(page)
		} else {
			tmpl, parseErr = template.ParseFiles(baseTmplPath, page)
		}

		if parseErr != nil {
			s.logger.Errorf("解析模板 %s 失败: %v", name, parseErr)
			continue
		}
		s.pageTemplates[name] = tmpl
		s.logger.Debugf("加载页面模板: %s", name)
	}
	s.logger.Infof("页面模板加载完成，共加载 %d 个页面", len(s.pageTemplates))
}

// setupRoutes 设置路由
func (s *Server) setupRoutes() {
	// 静态文件
	staticPath := s.config.Web.StaticPath
	if staticPath == "" {
		staticPath = "web/static"
	}
	s.router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir(staticPath))))

	// API 路由
	api := s.router.PathPrefix("/api").Subrouter()

	// 认证相关
	api.HandleFunc("/login", s.handleLogin).Methods("POST")
	api.HandleFunc("/logout", s.handleLogout).Methods("POST")
	api.HandleFunc("/auth/check", s.handleAuthCheck).Methods("GET")

	// 告警相关
	api.HandleFunc("/alerts", s.requireAuth(s.handleGetAlerts)).Methods("GET")
	api.HandleFunc("/alerts/stats", s.requireAuth(s.handleGetAlertStats)).Methods("GET")
	api.HandleFunc("/alerts/rule/{rule}", s.requireAuth(s.handleGetAlertsByRule)).Methods("GET")
	api.HandleFunc("/alerts/level/{level}", s.requireAuth(s.handleGetAlertsByLevel)).Methods("GET")
	api.HandleFunc("/alerts/{id}", s.requireAuth(s.handleGetAlertByID)).Methods("GET")

	// 规则相关
	api.HandleFunc("/rules", s.requireAuth(s.handleGetRules)).Methods("GET")
	api.HandleFunc("/rules", s.requireAuth(s.handleUpsertRule)).Methods("POST")
	api.HandleFunc("/rules/{name}/enable", s.requireAuth(s.handleEnableRule)).Methods("POST")
	api.HandleFunc("/rules/{name}/disable", s.requireAuth(s.handleDisableRule)).Methods("POST")

	// 配置相关
	api.HandleFunc("/config", s.requireAuth(s.handleGetConfig)).Methods("GET")
	api.HandleFunc("/config", s.requireAuth(s.handleUpdateConfig)).Methods("PUT")

	// 测试通知
	api.HandleFunc("/test/notification", s.requireAuth(s.handleTestNotification)).Methods("POST")

	// 页面路由
	s.router.HandleFunc("/", s.handleIndex).Methods("GET")
	s.router.HandleFunc("/login", s.handleLoginPage).Methods("GET")
	s.router.HandleFunc("/dashboard", s.requireAuth(s.handleDashboard)).Methods("GET")
	s.router.HandleFunc("/alerts", s.requireAuth(s.handleAlertsPage)).Methods("GET")
	s.router.HandleFunc("/rules", s.requireAuth(s.handleRulesPage)).Methods("GET")
	s.router.HandleFunc("/config", s.requireAuth(s.handleConfigPage)).Methods("GET")
}

// Start 启动 Web 服务器
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.Web.Host, s.config.Web.Port)
	s.logger.Infof("启动 Web 服务器: http://%s", addr)

	// 启动清理过期会话的定时任务
	go s.startSessionCleaner()

	return http.ListenAndServe(addr, s.router)
}

// startSessionCleaner 启动会话清理器
func (s *Server) startSessionCleaner() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		if err := s.database.CleanExpiredSessions(); err != nil {
			s.logger.Errorf("清理过期会话失败: %v", err)
		}
	}
}

// requireAuth 认证中间件
func (s *Server) requireAuth(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.config.Auth.Enabled {
			handler(w, r)
			return
		}

		session, _ := s.store.Get(r, "opensearch-alert-session")
		user := session.Values["user"]

		if user == nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		handler(w, r)
	}
}

// getCurrentUser 获取当前用户
func (s *Server) getCurrentUser(r *http.Request) *types.User {
	if !s.config.Auth.Enabled {
		return &types.User{Username: "admin", Role: "admin"}
	}

	session, _ := s.store.Get(r, "opensearch-alert-session")
	if user, ok := session.Values["user"].(*types.User); ok {
		return user
	}

	return nil
}

// handleIndex 首页
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	user := s.getCurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

// handleLoginPage 登录页面
func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	if tmpl, ok := s.pageTemplates["login.html"]; ok {
		if err := tmpl.ExecuteTemplate(w, "login.html", nil); err != nil {
			s.logger.Errorf("渲染登录页面失败: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	} else {
		s.logger.Errorf("登录页面模板未找到")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleLogin 处理登录
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req types.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondJSON(w, types.LoginResponse{
			Success: false,
			Message: "无效的请求格式",
		}, http.StatusBadRequest)
		return
	}

	// 验证用户
	var user *types.User
	for _, u := range s.config.Auth.Users {
		if u.Username == req.Username && u.Password == req.Password {
			user = &u
			break
		}
	}

	if user == nil {
		s.respondJSON(w, types.LoginResponse{
			Success: false,
			Message: "用户名或密码错误",
		}, http.StatusUnauthorized)
		return
	}

	// 创建会话（容错：旧密钥导致的解码失败时，创建新会话）
	session, err := s.store.Get(r, "opensearch-alert-session")
	if err != nil {
		s.logger.Warnf("获取会话失败，创建新会话: %v", err)
		session, _ = s.store.New(r, "opensearch-alert-session")
	}

	// 仅保存非敏感信息
	session.Values["user"] = &types.User{Username: user.Username, Role: user.Role}
	if err := session.Save(r, w); err != nil {
		s.logger.Errorf("保存会话失败: %v", err)
		s.respondJSON(w, types.LoginResponse{
			Success: false,
			Message: "会话保存失败",
		}, http.StatusInternalServerError)
		return
	}

	// 保存到数据库
	sessionID := session.ID
	expiresAt := time.Now().Add(time.Duration(s.config.Auth.SessionTimeout) * time.Second)
	if err := s.database.SaveSession(sessionID, user.Username, user.Role, expiresAt); err != nil {
		s.logger.Errorf("保存会话到数据库失败: %v", err)
		// 不返回错误，因为会话已经保存到cookie中
	}

	// 返回非敏感字段
	s.respondJSON(w, map[string]interface{}{
		"success": true,
		"message": "登录成功",
		"user":    map[string]string{"username": user.Username, "role": user.Role},
	}, http.StatusOK)
}

// handleLogout 处理登出
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	session, _ := s.store.Get(r, "opensearch-alert-session")
	sessionID := session.ID

	// 从数据库删除会话
	s.database.DeleteSession(sessionID)

	// 清除会话
	session.Values = make(map[interface{}]interface{})
	session.Options.MaxAge = -1
	session.Save(r, w)

	s.respondJSON(w, map[string]string{"message": "登出成功"}, http.StatusOK)
}

// handleAuthCheck 检查认证状态
func (s *Server) handleAuthCheck(w http.ResponseWriter, r *http.Request) {
	if !s.config.Auth.Enabled {
		s.respondJSON(w, map[string]interface{}{
			"authenticated": true,
			"user":          map[string]string{"username": "admin", "role": "admin"},
		}, http.StatusOK)
		return
	}

	session, _ := s.store.Get(r, "opensearch-alert-session")
	user := session.Values["user"]

	if user == nil {
		s.respondJSON(w, map[string]bool{"authenticated": false}, http.StatusUnauthorized)
		return
	}

	if u, ok := user.(*types.User); ok {
		s.respondJSON(w, map[string]interface{}{
			"authenticated": true,
			"user":          map[string]string{"username": u.Username, "role": u.Role},
		}, http.StatusOK)
		return
	}

	s.respondJSON(w, map[string]interface{}{
		"authenticated": true,
	}, http.StatusOK)
}

// handleDashboard Dashboard 页面
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	user := s.getCurrentUser(r)

	// 获取告警统计
	stats, err := s.database.GetAlertStats(24) // 最近24小时
	if err != nil {
		s.logger.Errorf("获取告警统计失败: %v", err)
		stats = &types.AlertStats{}
	}

	data := map[string]interface{}{
		"Title":      "Dashboard",
		"ActivePage": "dashboard",
		"AlertStats": *stats,
		"Config":     s.config,
		"User":       user,
	}

	if tmpl, ok := s.pageTemplates["dashboard.html"]; ok {
		if err := tmpl.ExecuteTemplate(w, "base.html", data); err != nil {
			s.logger.Errorf("渲染 Dashboard 页面失败: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	} else {
		s.logger.Errorf("Dashboard 页面模板未找到")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleAlertsPage 告警页面
func (s *Server) handleAlertsPage(w http.ResponseWriter, r *http.Request) {
	user := s.getCurrentUser(r)

	// 获取规则列表用于筛选
	rules, err := s.loadRules()
	if err != nil {
		s.logger.Errorf("加载规则失败: %v", err)
		rules = []types.AlertRule{}
	}

	data := map[string]interface{}{
		"Title":      "告警列表",
		"ActivePage": "alerts",
		"Rules":      rules,
		"User":       user,
	}

	if tmpl, ok := s.pageTemplates["alerts.html"]; ok {
		if err := tmpl.ExecuteTemplate(w, "base.html", data); err != nil {
			s.logger.Errorf("渲染告警页面失败: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	} else {
		s.logger.Errorf("告警页面模板未找到")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleRulesPage 规则页面
func (s *Server) handleRulesPage(w http.ResponseWriter, r *http.Request) {
	user := s.getCurrentUser(r)

	rules, err := s.loadRules()
	if err != nil {
		s.logger.Errorf("加载规则失败: %v", err)
		rules = []types.AlertRule{}
	}

	data := map[string]interface{}{
		"Title":      "规则管理",
		"ActivePage": "rules",
		"Rules":      rules,
		"User":       user,
	}

	if tmpl, ok := s.pageTemplates["rules.html"]; ok {
		if err := tmpl.ExecuteTemplate(w, "base.html", data); err != nil {
			s.logger.Errorf("渲染规则页面失败: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	} else {
		s.logger.Errorf("规则页面模板未找到")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleConfigPage 配置页面
func (s *Server) handleConfigPage(w http.ResponseWriter, r *http.Request) {
	user := s.getCurrentUser(r)

	// 检查权限
	if user.Role != "admin" {
		http.Error(w, "权限不足", http.StatusForbidden)
		return
	}

	data := map[string]interface{}{
		"Title":      "配置管理",
		"ActivePage": "config",
		"Config":     s.config,
		"User":       user,
	}

	if tmpl, ok := s.pageTemplates["config.html"]; ok {
		if err := tmpl.ExecuteTemplate(w, "base.html", data); err != nil {
			s.logger.Errorf("渲染配置页面失败: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	} else {
		s.logger.Errorf("配置页面模板未找到")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleGetAlerts 获取告警列表
func (s *Server) handleGetAlerts(w http.ResponseWriter, r *http.Request) {
	// 解析查询参数（旧逻辑：rule/level/hours + 分页）
	ruleName := r.URL.Query().Get("rule")
	level := r.URL.Query().Get("level")
	limitStr := r.URL.Query().Get("limit")
	pageStr := r.URL.Query().Get("page")
	pageSizeStr := r.URL.Query().Get("page_size")
	hoursStr := r.URL.Query().Get("hours")

	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	var alerts []types.AlertHistory
	var err error

	if ruleName != "" {
		alerts, err = s.database.GetAlertsByRule(ruleName, limit)
	} else if level != "" {
		alerts, err = s.database.GetAlertsByLevel(level, limit)
	} else {
		// 分页模式
		page, _ := strconv.Atoi(pageStr)
		pageSize, _ := strconv.Atoi(pageSizeStr)
		hours := 0
		if hoursStr != "" {
			if h, err := strconv.Atoi(hoursStr); err == nil {
				hours = h
			}
		}
		alerts, total, err := s.database.GetAlertsPaged(hours, page, pageSize)
		if err != nil {
			s.respondJSON(w, map[string]string{"error": "获取告警失败"}, http.StatusInternalServerError)
			return
		}
		s.respondJSON(w, map[string]interface{}{
			"alerts":    alerts,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		}, http.StatusOK)
		return
	}

	if err != nil {
		s.respondJSON(w, map[string]string{"error": "获取告警失败"}, http.StatusInternalServerError)
		return
	}

	s.respondJSON(w, map[string]interface{}{
		"alerts": alerts,
		"total":  len(alerts),
	}, http.StatusOK)
}

// handleGetAlertByID 根据ID获取告警详情
func (s *Server) handleGetAlertByID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		s.respondJSON(w, map[string]string{"error": "缺少告警ID"}, http.StatusBadRequest)
		return
	}

	detail, err := s.database.GetAlertByID(id)
	if err != nil {
		s.respondJSON(w, map[string]string{"error": "获取告警详情失败"}, http.StatusInternalServerError)
		return
	}
	if detail == nil {
		s.respondJSON(w, map[string]string{"error": "未找到该告警"}, http.StatusNotFound)
		return
	}

	s.respondJSON(w, detail, http.StatusOK)
}

// handleGetAlertStats 获取告警统计
func (s *Server) handleGetAlertStats(w http.ResponseWriter, r *http.Request) {
	hoursStr := r.URL.Query().Get("hours")
	hours := 24
	if hoursStr != "" {
		if h, err := strconv.Atoi(hoursStr); err == nil {
			hours = h
		}
	}

	stats, err := s.database.GetAlertStats(hours)
	if err != nil {
		s.respondJSON(w, map[string]string{"error": "获取统计失败"}, http.StatusInternalServerError)
		return
	}

	s.respondJSON(w, stats, http.StatusOK)
}

// handleGetAlertsByRule 根据规则获取告警
func (s *Server) handleGetAlertsByRule(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	ruleName := vars["rule"]

	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	alerts, err := s.database.GetAlertsByRule(ruleName, limit)
	if err != nil {
		s.respondJSON(w, map[string]string{"error": "获取告警失败"}, http.StatusInternalServerError)
		return
	}

	s.respondJSON(w, map[string]interface{}{
		"alerts": alerts,
		"total":  len(alerts),
	}, http.StatusOK)
}

// handleGetAlertsByLevel 根据级别获取告警
func (s *Server) handleGetAlertsByLevel(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	level := vars["level"]

	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	alerts, err := s.database.GetAlertsByLevel(level, limit)
	if err != nil {
		s.respondJSON(w, map[string]string{"error": "获取告警失败"}, http.StatusInternalServerError)
		return
	}

	s.respondJSON(w, map[string]interface{}{
		"alerts": alerts,
		"total":  len(alerts),
	}, http.StatusOK)
}

// handleGetRules 获取规则列表
func (s *Server) handleGetRules(w http.ResponseWriter, r *http.Request) {
	rules, err := s.loadRules()
	if err != nil {
		s.respondJSON(w, map[string]string{"error": "获取规则失败"}, http.StatusInternalServerError)
		return
	}

	s.respondJSON(w, map[string]interface{}{
		"rules": rules,
		"total": len(rules),
	}, http.StatusOK)
}

// handleEnableRule 启用规则（修改规则文件 enabled:true）
func (s *Server) handleEnableRule(w http.ResponseWriter, r *http.Request) {
	user := s.getCurrentUser(r)
	if user == nil || user.Role != "admin" {
		s.respondJSON(w, map[string]string{"error": "权限不足"}, http.StatusForbidden)
		return
	}
	vars := mux.Vars(r)
	name := vars["name"]
	if err := s.updateRuleEnabled(name, true); err != nil {
		s.respondJSON(w, map[string]string{"error": err.Error()}, http.StatusInternalServerError)
		return
	}
	s.reloadRules()
	s.respondJSON(w, map[string]string{"message": "规则已启用"}, http.StatusOK)
}

// handleDisableRule 禁用规则（修改规则文件 enabled:false）
func (s *Server) handleDisableRule(w http.ResponseWriter, r *http.Request) {
	user := s.getCurrentUser(r)
	if user == nil || user.Role != "admin" {
		s.respondJSON(w, map[string]string{"error": "权限不足"}, http.StatusForbidden)
		return
	}
	vars := mux.Vars(r)
	name := vars["name"]
	if err := s.updateRuleEnabled(name, false); err != nil {
		s.respondJSON(w, map[string]string{"error": err.Error()}, http.StatusInternalServerError)
		return
	}
	s.reloadRules()
	s.respondJSON(w, map[string]string{"message": "规则已禁用"}, http.StatusOK)
}

// updateRuleEnabled 在规则目录中查找匹配名称的 YAML 并更新 enabled 字段
func (s *Server) updateRuleEnabled(ruleName string, enabled bool) error {
	rulesDir := s.config.Rules.RulesFolder
	if rulesDir == "" {
		rulesDir = "configs/rules"
	}

	files, err := filepath.Glob(filepath.Join(rulesDir, "*.yaml"))
	if err != nil {
		return fmt.Errorf("读取规则目录失败: %w", err)
	}

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		var rule types.AlertRule
		if err := yaml.Unmarshal(data, &rule); err != nil {
			continue
		}

		if rule.Name == ruleName {
			rule.Enabled = enabled
			out, err := yaml.Marshal(&rule)
			if err != nil {
				return fmt.Errorf("序列化规则失败: %w", err)
			}
			if err := os.WriteFile(file, out, 0644); err != nil {
				return fmt.Errorf("写入规则文件失败: %w", err)
			}
			// 重新加载模板缓存的页面数据使用最新规则数量
			return nil
		}
	}
	return fmt.Errorf("未找到规则: %s", ruleName)
}

// handleUpsertRule 新增或更新规则（根据 Name 匹配文件名；若存在则覆盖，不存在则创建）
func (s *Server) handleUpsertRule(w http.ResponseWriter, r *http.Request) {
	user := s.getCurrentUser(r)
	if user.Role != "admin" {
		s.respondJSON(w, map[string]string{"error": "权限不足"}, http.StatusForbidden)
		return
	}

	// 读取原始请求体，便于判断哪些字段真实提供了
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.respondJSON(w, map[string]string{"error": "读取请求失败"}, http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var rule types.AlertRule
	if err := json.Unmarshal(body, &rule); err != nil {
		s.respondJSON(w, map[string]string{"error": "无效的规则格式"}, http.StatusBadRequest)
		return
	}
	// 记录原始键，用于保留未提交字段
	var raw map[string]interface{}
	_ = json.Unmarshal(body, &raw)
	if rule.Name == "" {
		s.respondJSON(w, map[string]string{"error": "规则名称不能为空"}, http.StatusBadRequest)
		return
	}

	rulesDir := s.config.Rules.RulesFolder
	if rulesDir == "" {
		rulesDir = "configs/rules"
	}
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		s.respondJSON(w, map[string]string{"error": "创建规则目录失败"}, http.StatusInternalServerError)
		return
	}

	// 尝试在目录中查找同名规则的现有文件
	var rulePath string
	files, _ := filepath.Glob(filepath.Join(rulesDir, "*.yaml"))
	var existing types.AlertRule
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		if err := yaml.Unmarshal(b, &existing); err != nil {
			continue
		}
		if existing.Name == rule.Name {
			rulePath = f
			break
		}
	}
	// 若未找到，则以规则名称生成安全的新文件名
	if rulePath == "" {
		fileName := fmt.Sprintf("%s.yaml", rule.Name)
		fileName = strings.ReplaceAll(fileName, "/", "_")
		fileName = strings.ReplaceAll(fileName, "\\", "_")
		rulePath = filepath.Join(rulesDir, fileName)
	}

	// 合并：对未在请求中出现的字段，保留旧文件中的值
	if rulePath != "" && existing.Name == rule.Name {
		if _, ok := raw["QueryKey"]; !ok && len(rule.QueryKey) == 0 && len(existing.QueryKey) > 0 {
			rule.QueryKey = existing.QueryKey
		}
		if _, ok := raw["Alert"]; !ok && len(rule.Alert) == 0 && len(existing.Alert) > 0 {
			rule.Alert = existing.Alert
		}
		if _, ok := raw["AlertTextArgs"]; !ok && len(rule.AlertTextArgs) == 0 && len(existing.AlertTextArgs) > 0 {
			rule.AlertTextArgs = existing.AlertTextArgs
		}
	}
	data, err := yaml.Marshal(&rule)
	if err != nil {
		s.respondJSON(w, map[string]string{"error": "序列化规则失败"}, http.StatusInternalServerError)
		return
	}
	if err := os.WriteFile(rulePath, data, 0644); err != nil {
		s.respondJSON(w, map[string]string{"error": "写入规则文件失败"}, http.StatusInternalServerError)
		return
	}

	// 保存完成后热加载规则
	s.reloadRules()

	s.respondJSON(w, map[string]string{"message": "规则保存成功"}, http.StatusOK)
}

// handleGetConfig 获取配置
func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	user := s.getCurrentUser(r)
	if user.Role != "admin" {
		s.respondJSON(w, map[string]string{"error": "权限不足"}, http.StatusForbidden)
		return
	}

	// 转换为前端期望的小写键名结构
	cfg := s.config
	apiConfig := map[string]interface{}{
		"opensearch": map[string]interface{}{
			"host":         cfg.OpenSearch.Host,
			"port":         cfg.OpenSearch.Port,
			"protocol":     cfg.OpenSearch.Protocol,
			"username":     cfg.OpenSearch.Username,
			"password":     cfg.OpenSearch.Password,
			"verify_certs": cfg.OpenSearch.VerifyCerts,
			"timeout":      cfg.OpenSearch.Timeout,
		},
		"alert_engine": map[string]interface{}{
			"run_interval":      cfg.AlertEngine.RunInterval,
			"buffer_time":       cfg.AlertEngine.BufferTime,
			"max_running_rules": cfg.AlertEngine.MaxRunningRules,
			"writeback_index":   cfg.AlertEngine.WritebackIndex,
			"alert_time_limit":  cfg.AlertEngine.AlertTimeLimit,
		},
		"web": map[string]interface{}{
			"enabled":        cfg.Web.Enabled,
			"host":           cfg.Web.Host,
			"port":           cfg.Web.Port,
			"static_path":    cfg.Web.StaticPath,
			"template_path":  cfg.Web.TemplatePath,
			"session_secret": cfg.Web.SessionSecret,
		},
		"database": map[string]interface{}{
			"type":                 cfg.Database.Type,
			"path":                 cfg.Database.Path,
			"max_connections":      cfg.Database.MaxConnections,
			"max_idle_connections": cfg.Database.MaxIdleConnections,
			"host":                 cfg.Database.Host,
			"port":                 cfg.Database.Port,
			"username":             cfg.Database.Username,
			"password":             cfg.Database.Password,
			"dbname":               cfg.Database.DBName,
			"params":               cfg.Database.Params,
		},
		"notifications": map[string]interface{}{
			"email": map[string]interface{}{
				"enabled":     cfg.Notifications.Email.Enabled,
				"smtp_server": cfg.Notifications.Email.SMTPServer,
				"smtp_port":   cfg.Notifications.Email.SMTPPort,
				"username":    cfg.Notifications.Email.Username,
				"password":    cfg.Notifications.Email.Password,
				"from_email":  cfg.Notifications.Email.FromEmail,
				"to_emails":   cfg.Notifications.Email.ToEmails,
				"use_tls":     cfg.Notifications.Email.UseTLS,
			},
			"dingtalk": map[string]interface{}{
				"enabled":     cfg.Notifications.DingTalk.Enabled,
				"webhook_url": cfg.Notifications.DingTalk.WebhookURL,
				"secret":      cfg.Notifications.DingTalk.Secret,
				"at_mobiles":  cfg.Notifications.DingTalk.AtMobiles,
				"at_all":      cfg.Notifications.DingTalk.AtAll,
			},
			"wechat": map[string]interface{}{
				"enabled":               cfg.Notifications.WeChat.Enabled,
				"webhook_url":           cfg.Notifications.WeChat.WebhookURL,
				"mentioned_list":        cfg.Notifications.WeChat.MentionedList,
				"mentioned_mobile_list": cfg.Notifications.WeChat.MentionedMobileList,
				"at_all":                cfg.Notifications.WeChat.AtAll,
			},
			"feishu": map[string]interface{}{
				"enabled":     cfg.Notifications.Feishu.Enabled,
				"webhook_url": cfg.Notifications.Feishu.WebhookURL,
				"secret":      cfg.Notifications.Feishu.Secret,
				"at_mobiles":  cfg.Notifications.Feishu.AtMobiles,
				"at_all":      cfg.Notifications.Feishu.AtAll,
			},
		},
	}

	s.respondJSON(w, apiConfig, http.StatusOK)
}

// handleUpdateConfig 更新配置
func (s *Server) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	user := s.getCurrentUser(r)
	if user.Role != "admin" {
		s.respondJSON(w, map[string]string{"error": "权限不足"}, http.StatusForbidden)
		return
	}

	// 1) 读取前端发来的 JSON（蛇形键名）到通用 map
	var payload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		s.respondJSON(w, map[string]string{"error": "无效的配置格式"}, http.StatusBadRequest)
		return
	}

	// 2) 借助 YAML 标签把 map 映射为 types.Config（避免手动逐字段转换）
	yamlBytes, err := yaml.Marshal(payload)
	if err != nil {
		s.respondJSON(w, map[string]string{"error": "配置编码失败"}, http.StatusBadRequest)
		return
	}

	var newCfg types.Config
	if err := yaml.Unmarshal(yamlBytes, &newCfg); err != nil {
		s.respondJSON(w, map[string]string{"error": "配置解析失败"}, http.StatusBadRequest)
		return
	}

	// 3) 合并到现有配置（仅覆盖前端可编辑的部分）
	s.config.OpenSearch = newCfg.OpenSearch
	s.config.AlertEngine = newCfg.AlertEngine
	s.config.Web = newCfg.Web
	s.config.Database = newCfg.Database
	s.config.Notifications = newCfg.Notifications

	// 4) 落盘持久化到配置文件
	if err := s.saveConfigToFile(); err != nil {
		s.logger.Errorf("保存配置到文件失败: %v", err)
		s.respondJSON(w, map[string]string{"error": "保存配置到文件失败"}, http.StatusInternalServerError)
		return
	}

	s.respondJSON(w, map[string]string{"message": "配置更新成功"}, http.StatusOK)
}

// saveConfigToFile 将当前内存配置写回 YAML 文件，实现持久化
func (s *Server) saveConfigToFile() error {
	// 优先使用环境变量指定路径，其次使用默认路径
	configPath := os.Getenv("OPENSEARCH_ALERT_CONFIG_PATH")
	if configPath == "" {
		configPath = "configs/config.yaml"
	}

	data, err := yaml.Marshal(s.config)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}
	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}
	s.logger.Infof("配置已保存到: %s", configPath)
	return nil
}

// handleTestNotification 测试通知
func (s *Server) handleTestNotification(w http.ResponseWriter, r *http.Request) {
	user := s.getCurrentUser(r)
	if user.Role != "admin" {
		s.respondJSON(w, map[string]string{"error": "权限不足"}, http.StatusForbidden)
		return
	}

	// 创建测试告警
	testAlert := &types.Alert{
		ID:        fmt.Sprintf("test-web-%d", time.Now().Unix()),
		RuleName:  "Web 测试告警",
		Level:     "Info",
		Message:   "这是一条通过 Web 界面发送的测试告警消息。",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"test":    true,
			"source":  "web",
			"message": "Web 界面测试成功",
		},
		Count:   1,
		Matches: 1,
	}

	// 发送通知
	if err := s.notifier.SendAlert(testAlert); err != nil {
		s.respondJSON(w, map[string]string{"error": "发送测试通知失败"}, http.StatusInternalServerError)
		return
	}

	// 保存到数据库
	s.database.SaveAlert(testAlert)

	s.respondJSON(w, map[string]string{"message": "测试通知发送成功"}, http.StatusOK)
}

// loadRules 加载规则
func (s *Server) loadRules() ([]types.AlertRule, error) {
	// 加载所有规则（包含禁用规则）
	rulesDir := s.config.Rules.RulesFolder
	if rulesDir == "" {
		rulesDir = "configs/rules"
	}

	files, err := filepath.Glob(filepath.Join(rulesDir, "*.yaml"))
	if err != nil {
		s.logger.Errorf("读取规则目录失败: %v", err)
		return []types.AlertRule{}, err
	}

	// 按规则名称去重：同名规则仅保留最近修改的文件
	type ruleWithMeta struct {
		rule  types.AlertRule
		mtime time.Time
	}
	nameToRule := make(map[string]ruleWithMeta)
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			s.logger.Warnf("读取规则文件失败: %s: %v", file, err)
			continue
		}
		var rule types.AlertRule
		if err := yaml.Unmarshal(data, &rule); err != nil {
			s.logger.Warnf("解析规则文件失败: %s: %v", file, err)
			continue
		}
		// 兜底：如果 Threshold 为 0 而 YAML 中确有 threshold 值，直接读取原始 YAML 再次解析该键
		if rule.Threshold == 0 {
			var raw map[string]interface{}
			if err := yaml.Unmarshal(data, &raw); err == nil {
				if tv, ok := raw["threshold"].(int); ok {
					rule.Threshold = tv
				} else if fv, ok := raw["threshold"].(float64); ok {
					rule.Threshold = int(fv)
				}
			}
		}
		fi, _ := os.Stat(file)
		meta := ruleWithMeta{rule: rule, mtime: time.Time{}}
		if fi != nil {
			meta.mtime = fi.ModTime()
		}
		if exist, ok := nameToRule[rule.Name]; ok {
			// 取最近修改的一个
			if meta.mtime.After(exist.mtime) {
				nameToRule[rule.Name] = meta
			}
		} else {
			nameToRule[rule.Name] = meta
		}
	}
	// 转为切片
	rules := make([]types.AlertRule, 0, len(nameToRule))
	for _, v := range nameToRule {
		rules = append(rules, v.rule)
	}
	return rules, nil
}

// reloadRules 从当前规则目录加载并应用到告警引擎（填充默认值）
func (s *Server) reloadRules() {
	if s.engine == nil {
		return
	}
	rules, err := s.loadRules()
	if err != nil {
		s.logger.Errorf("热加载规则失败: %v", err)
		return
	}
	// 回填默认值
	for i := range rules {
		if rules[i].Timeframe == 0 {
			rules[i].Timeframe = s.config.Rules.DefaultTimeframe
		}
		if rules[i].Threshold == 0 {
			rules[i].Threshold = s.config.Rules.DefaultThreshold
		}
	}
	s.engine.LoadRules(rules)
	s.logger.Infof("规则热加载完成: %d 条", len(rules))
}

// respondJSON 响应 JSON
func (s *Server) respondJSON(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

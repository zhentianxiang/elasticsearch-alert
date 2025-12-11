/**
 * 配置页面 JavaScript
 */

class ConfigPage {
    constructor() {
        this.currentConfig = null;
        this.init();
    }

    // 初始化
    init() {
        this.setupEventListeners();
        this.loadConfig();
    }

    // 设置事件监听器
    setupEventListeners() {
        // 刷新按钮
        const refreshBtn = document.querySelector('[onclick="loadConfig()"]');
        if (refreshBtn) {
            refreshBtn.addEventListener('click', (e) => {
                e.preventDefault();
                this.loadConfig();
            });
        }

        // 保存配置按钮
        const saveBtn = document.querySelector('[onclick="saveConfig()"]');
        if (saveBtn) {
            saveBtn.addEventListener('click', (e) => {
                e.preventDefault();
                this.showEditConfigModal();
            });
        }

        // 保存配置表单
        const saveFormBtn = document.querySelector('[onclick="saveConfigForm()"]');
        if (saveFormBtn) {
            saveFormBtn.addEventListener('click', (e) => {
                e.preventDefault();
                this.saveConfigForm();
            });
        }

        // 切换数据库类型时动态显示字段
        document.addEventListener('change', (e) => {
            const sel = e.target;
            if (sel && sel.id === 'dbType') {
                if (window.configPage) {
                    window.configPage.toggleDBFields(sel.value);
                }
            }
        });
    }

    // 加载配置数据
    async loadConfig() {
        try {
            Loading.show('configContainer', '加载配置数据中...');
            
            const data = await API.get('/config');
            this.currentConfig = data;
            
            this.displayConfig(data);
        } catch (error) {
            console.error('加载配置失败:', error);
            Loading.error('configContainer', '加载配置失败: ' + error.message);
        }
    }

    // 显示配置信息
    displayConfig(config) {
        const container = document.getElementById('configContainer');
        if (!container) return;

        let html = `
            <div class="row">
                <div class="col-md-6">
                    <h6>OpenSearch 配置</h6>
                    <table class="table table-sm">
                        <tr><td>主机地址:</td><td><code>${config.opensearch?.host || 'N/A'}</code></td></tr>
                        <tr><td>端口:</td><td>${config.opensearch?.port || 'N/A'}</td></tr>
                        <tr><td>协议:</td><td>${config.opensearch?.protocol || 'N/A'}</td></tr>
                        <tr><td>用户名:</td><td>${config.opensearch?.username || 'N/A'}</td></tr>
                        <tr><td>超时时间:</td><td>${config.opensearch?.timeout || 'N/A'}秒</td></tr>
                    </table>
                </div>
                <div class="col-md-6">
                    <h6>告警引擎配置</h6>
                    <table class="table table-sm">
                        <tr><td>检查间隔:</td><td>${config.alert_engine?.run_interval || 'N/A'}秒</td></tr>
                        <tr><td>缓冲时间:</td><td>${config.alert_engine?.buffer_time || 'N/A'}秒</td></tr>
                        <tr><td>最大并发规则数:</td><td>${config.alert_engine?.max_running_rules || 'N/A'}</td></tr>
                        <tr><td>状态索引:</td><td><code>${config.alert_engine?.writeback_index || 'N/A'}</code></td></tr>
                        <tr><td>告警保留时间:</td><td>${config.alert_engine?.alert_time_limit || 'N/A'}秒</td></tr>
                    </table>
                </div>
            </div>
            
            <div class="row mt-4">
                <div class="col-md-6">
                    <h6>Web 服务配置</h6>
                    <table class="table table-sm">
                        <tr><td>启用状态:</td><td><span class="badge bg-${config.web?.enabled ? 'success' : 'danger'}">${config.web?.enabled ? '启用' : '禁用'}</span></td></tr>
                        <tr><td>监听地址:</td><td><code>${config.web?.host || 'N/A'}</code></td></tr>
                        <tr><td>端口:</td><td>${config.web?.port || 'N/A'}</td></tr>
                    </table>
                </div>
                <div class="col-md-6">
                    <h6>数据库配置</h6>
                    <table class="table table-sm">
                        <tr><td>数据库类型:</td><td>${config.database?.type || 'N/A'}</td></tr>
                        ${(config.database?.type === 'mysql') ? `
                        <tr><td>主机:</td><td>${config.database?.host || 'N/A'}:${config.database?.port || 3306}</td></tr>
                        <tr><td>用户名:</td><td>${config.database?.username || 'N/A'}</td></tr>
                        <tr><td>数据库名:</td><td>${config.database?.dbname || 'N/A'}</td></tr>
                        ` : `
                        <tr><td>数据库路径:</td><td><code>${config.database?.path || 'N/A'}</code></td></tr>
                        `}
                        <tr><td>最大连接数:</td><td>${config.database?.max_connections || 'N/A'}</td></tr>
                    </table>
                </div>
            </div>
            
            <div class="row mt-4">
                <div class="col-12">
                    <h6>通知渠道配置</h6>
                    <div class="row">
                        <div class="col-md-3">
                            <div class="card text-center">
                                <div class="card-body">
                                    <h6 class="card-title">邮件通知</h6>
                                    <span class="badge bg-${config.notifications?.email?.enabled ? 'success' : 'danger'}">
                                        ${config.notifications?.email?.enabled ? '启用' : '禁用'}
                                    </span>
                                </div>
                            </div>
                        </div>
                        <div class="col-md-3">
                            <div class="card text-center">
                                <div class="card-body">
                                    <h6 class="card-title">钉钉通知</h6>
                                    <span class="badge bg-${config.notifications?.dingtalk?.enabled ? 'success' : 'danger'}">
                                        ${config.notifications?.dingtalk?.enabled ? '启用' : '禁用'}
                                    </span>
                                </div>
                            </div>
                        </div>
                        <div class="col-md-3">
                            <div class="card text-center">
                                <div class="card-body">
                                    <h6 class="card-title">企业微信通知</h6>
                                    <span class="badge bg-${config.notifications?.wechat?.enabled ? 'success' : 'danger'}">
                                        ${config.notifications?.wechat?.enabled ? '启用' : '禁用'}
                                    </span>
                                </div>
                            </div>
                        </div>
                        <div class="col-md-3">
                            <div class="card text-center">
                                <div class="card-body">
                                    <h6 class="card-title">飞书通知</h6>
                                    <span class="badge bg-${config.notifications?.feishu?.enabled ? 'success' : 'danger'}">
                                        ${config.notifications?.feishu?.enabled ? '启用' : '禁用'}
                                    </span>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
            
            <div class="row mt-4">
                <div class="col-12 text-center">
                    <button class="btn btn-primary" onclick="showEditConfigModal()">
                        <i class="bi bi-pencil"></i> 编辑配置
                    </button>
                </div>
            </div>
        `;

        container.innerHTML = html;
    }

    // 显示编辑配置模态框
    showEditConfigModal() {
        if (!this.currentConfig) return;
        
        const modal = document.getElementById('configEditModal');
        if (!modal) return;

        this.populateConfigForm(this.currentConfig);
        
        const bootstrapModal = new bootstrap.Modal(modal);
        bootstrapModal.show();
    }

    // 填充配置表单
    populateConfigForm(config) {
        // OpenSearch 配置
        document.getElementById('opensearchHost').value = config.opensearch?.host || '';
        document.getElementById('opensearchPort').value = config.opensearch?.port || 9200;
        document.getElementById('opensearchProtocol').value = config.opensearch?.protocol || 'https';
        document.getElementById('opensearchUsername').value = config.opensearch?.username || '';
        document.getElementById('opensearchPassword').value = config.opensearch?.password || '';
        
        // 告警引擎配置
        document.getElementById('runInterval').value = config.alert_engine?.run_interval || 60;
        document.getElementById('bufferTime').value = config.alert_engine?.buffer_time || 300;
        document.getElementById('maxRunningRules').value = config.alert_engine?.max_running_rules || 10;
        document.getElementById('writebackIndex').value = config.alert_engine?.writeback_index || '';
        document.getElementById('alertTimeLimit').value = config.alert_engine?.alert_time_limit || 172800;
        
        // Web 服务配置
        document.getElementById('webEnabled').checked = config.web?.enabled || false;
        document.getElementById('webHost').value = config.web?.host || '0.0.0.0';
        document.getElementById('webPort').value = config.web?.port || 8080;
        
        // 数据库配置
        const dbTypeSel = document.getElementById('dbType');
        dbTypeSel.value = config.database?.type || 'sqlite';
        document.getElementById('dbPath').value = config.database?.path || '';
        document.getElementById('maxConnections').value = config.database?.max_connections || 10;
        // MySQL 字段（若表单存在则填充）
        const el = (id) => document.getElementById(id);
        if (el('mysqlHost')) el('mysqlHost').value = config.database?.host || '';
        if (el('mysqlPort')) el('mysqlPort').value = config.database?.port || 3306;
        if (el('mysqlUser')) el('mysqlUser').value = config.database?.username || '';
        if (el('mysqlPass')) el('mysqlPass').value = config.database?.password || '';
        if (el('mysqlDB')) el('mysqlDB').value = config.database?.dbname || '';
        if (el('mysqlParams')) el('mysqlParams').value = config.database?.params || '';
        this.toggleDBFields(dbTypeSel.value);
        
        // 通知渠道配置
        document.getElementById('emailEnabled').checked = config.notifications?.email?.enabled || false;
        document.getElementById('smtpServer').value = config.notifications?.email?.smtp_server || '';
        document.getElementById('smtpPort').value = config.notifications?.email?.smtp_port || 587;
        document.getElementById('smtpUsername').value = config.notifications?.email?.username || '';
        document.getElementById('smtpPassword').value = config.notifications?.email?.password || '';
        document.getElementById('fromEmail').value = config.notifications?.email?.from_email || '';
        
        document.getElementById('dingtalkEnabled').checked = config.notifications?.dingtalk?.enabled || false;
        document.getElementById('dingtalkWebhook').value = config.notifications?.dingtalk?.webhook_url || '';
        document.getElementById('dingtalkSecret').value = config.notifications?.dingtalk?.secret || '';
        document.getElementById('dingtalkAtMobiles').value = (config.notifications?.dingtalk?.at_mobiles || []).join(',');
        document.getElementById('dingtalkAtAll').checked = config.notifications?.dingtalk?.at_all || false;
        
        document.getElementById('wechatEnabled').checked = config.notifications?.wechat?.enabled || false;
        document.getElementById('wechatWebhook').value = config.notifications?.wechat?.webhook_url || '';
        document.getElementById('wechatMentionedMobiles').value = (config.notifications?.wechat?.mentioned_mobile_list || []).join(',');
        document.getElementById('wechatAtAll').checked = config.notifications?.wechat?.at_all || false;
        
        document.getElementById('feishuEnabled').checked = config.notifications?.feishu?.enabled || false;
        document.getElementById('feishuWebhook').value = config.notifications?.feishu?.webhook_url || '';
        document.getElementById('feishuSecret').value = config.notifications?.feishu?.secret || '';
        document.getElementById('feishuAtMobiles').value = (config.notifications?.feishu?.at_mobiles || []).join(',');
        document.getElementById('feishuAtAll').checked = config.notifications?.feishu?.at_all || false;
    }

    // 保存配置表单
    async saveConfigForm() {
        try {
            const newConfig = this.buildConfigFromForm();
            
            const response = await API.put('/config', newConfig);
            if (response.message) {
                Notification.success(response.message);
                const modal = bootstrap.Modal.getInstance(document.getElementById('configEditModal'));
                modal.hide();
                // 刷新配置显示
                setTimeout(() => this.loadConfig(), 1000);
            } else {
                Notification.error(response.error || '保存配置失败');
            }
        } catch (error) {
            console.error('保存配置失败:', error);
            Notification.error('保存配置失败: ' + error.message);
        }
    }

    // 从表单构建配置对象
    buildConfigFromForm() {
        return {
            opensearch: {
                host: document.getElementById('opensearchHost').value,
                port: parseInt(document.getElementById('opensearchPort').value),
                protocol: document.getElementById('opensearchProtocol').value,
                username: document.getElementById('opensearchUsername').value,
                password: document.getElementById('opensearchPassword').value,
                verify_certs: false,
                timeout: 30
            },
            alert_engine: {
                run_interval: parseInt(document.getElementById('runInterval').value),
                buffer_time: parseInt(document.getElementById('bufferTime').value),
                max_running_rules: parseInt(document.getElementById('maxRunningRules').value),
                writeback_index: document.getElementById('writebackIndex').value,
                alert_time_limit: parseInt(document.getElementById('alertTimeLimit').value)
            },
            web: {
                enabled: document.getElementById('webEnabled').checked,
                host: document.getElementById('webHost').value,
                port: parseInt(document.getElementById('webPort').value),
                static_path: "web/static",
                template_path: "web/templates",
                session_secret: "opensearch-alert-secret-key-2024"
            },
            database: {
                type: document.getElementById('dbType').value,
                path: document.getElementById('dbPath').value,
                max_connections: parseInt(document.getElementById('maxConnections').value),
                max_idle_connections: 5,
                host: (document.getElementById('mysqlHost')||{}).value,
                port: parseInt((document.getElementById('mysqlPort')||{value:3306}).value),
                username: (document.getElementById('mysqlUser')||{}).value,
                password: (document.getElementById('mysqlPass')||{}).value,
                dbname: (document.getElementById('mysqlDB')||{}).value,
                params: (document.getElementById('mysqlParams')||{}).value
            },
            notifications: {
                email: {
                    enabled: document.getElementById('emailEnabled').checked,
                    smtp_server: document.getElementById('smtpServer').value,
                    smtp_port: parseInt(document.getElementById('smtpPort').value),
                    username: document.getElementById('smtpUsername').value,
                    password: document.getElementById('smtpPassword').value,
                    from_email: document.getElementById('fromEmail').value,
                    to_emails: [],
                    use_tls: true
                },
                dingtalk: {
                    enabled: document.getElementById('dingtalkEnabled').checked,
                    webhook_url: document.getElementById('dingtalkWebhook').value,
                    secret: document.getElementById('dingtalkSecret').value,
                    at_mobiles: document.getElementById('dingtalkAtMobiles').value.split(',').filter(s => s.trim()),
                    at_all: document.getElementById('dingtalkAtAll').checked
                },
                wechat: {
                    enabled: document.getElementById('wechatEnabled').checked,
                    webhook_url: document.getElementById('wechatWebhook').value,
                    mentioned_list: [],
                    mentioned_mobile_list: document.getElementById('wechatMentionedMobiles').value.split(',').filter(s => s.trim()),
                    at_all: document.getElementById('wechatAtAll').checked
                },
                feishu: {
                    enabled: document.getElementById('feishuEnabled').checked,
                    webhook_url: document.getElementById('feishuWebhook').value,
                    secret: document.getElementById('feishuSecret').value,
                    at_mobiles: document.getElementById('feishuAtMobiles').value.split(',').filter(s => s.trim()),
                    at_all: document.getElementById('feishuAtAll').checked
                }
            }
        };
    }

    // 根据数据库类型显示/隐藏对应字段
    toggleDBFields(type) {
        const mysqlGroup = document.getElementById('mysqlFields');
        const pathGroup = document.getElementById('dbPathGroup');
        if (!mysqlGroup || !pathGroup) return;
        if (type === 'mysql') {
            mysqlGroup.classList.remove('d-none');
            pathGroup.classList.add('d-none');
        } else {
            mysqlGroup.classList.add('d-none');
            pathGroup.classList.remove('d-none');
        }
    }

    // 保存配置
    saveConfig() {
        this.showEditConfigModal();
    }
}

// 页面特定的函数
function loadConfig() {
    if (window.configPage) {
        window.configPage.loadConfig();
    }
}

function saveConfig() {
    if (window.configPage) {
        window.configPage.saveConfig();
    }
}

function showEditConfigModal() {
    if (window.configPage) {
        window.configPage.showEditConfigModal();
    }
}

function saveConfigForm() {
    if (window.configPage) {
        window.configPage.saveConfigForm();
    }
}

// 页面加载完成后初始化
document.addEventListener('DOMContentLoaded', function() {
    // 检查是否在配置页面
    if (document.getElementById('configContainer')) {
        window.configPage = new ConfigPage();
    }
});

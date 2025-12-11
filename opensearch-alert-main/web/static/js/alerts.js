/**
 * 告警页面 JavaScript
 */

class AlertsPage {
    constructor() {
        this.currentFilters = { rule: '', level: '', time: 24 };
        this.refreshTimer = null;
        this.init();
    }

    // 初始化
    init() {
        this.setupEventListeners();
        this.loadAlerts();
        this.startAutoRefresh();
    }

    // 设置事件监听器
    setupEventListeners() {
        // 筛选按钮
        const filterBtn = document.querySelector('[onclick="applyFilters()"]');
        if (filterBtn) {
            filterBtn.addEventListener('click', (e) => {
                e.preventDefault();
                this.applyFilters();
            });
        }

        // 刷新按钮
        const refreshBtn = document.querySelector('[onclick="refreshAlerts()"]');
        if (refreshBtn) {
            refreshBtn.addEventListener('click', (e) => {
                e.preventDefault();
                this.loadAlerts();
            });
        }

        // 测试通知按钮
        const testBtn = document.querySelector('[onclick="testNotification()"]');
        if (testBtn) {
            testBtn.addEventListener('click', (e) => {
                e.preventDefault();
                this.testNotification();
            });
        }

        // 筛选器变化
        const filters = ['ruleFilter', 'levelFilter', 'timeFilter'];
        filters.forEach(filterId => {
            const element = document.getElementById(filterId);
            if (element) {
                element.addEventListener('change', () => {
                    this.updateFilters();
                });
            }
        });
    }

    // 加载告警数据（默认分页）
    async loadAlerts() {
        try {
            Loading.show('alertsContainer', '加载告警数据中...');
            
            const url = this.buildAlertsUrl();
            const data = await API.get(url);
            
            this.displayAlerts(data.alerts || []);
            this.renderPagination(data.total || 0, data.page || 1, data.page_size || 10);
        } catch (error) {
            console.error('加载告警失败:', error);
            Loading.error('alertsContainer', '加载告警失败: ' + error.message);
        }
    }

    // 构建告警URL
    buildAlertsUrl() {
        const params = new URLSearchParams();
        
        if (this.currentFilters.rule) {
            params.append('rule', this.currentFilters.rule);
        }
        if (this.currentFilters.level) {
            params.append('level', this.currentFilters.level);
        }
        // 无关键词/起止时间参数（恢复旧逻辑）
        // 分页参数
        params.append('page', this.currentPage || 1);
        params.append('page_size', this.pageSize || 10);
        if (this.currentFilters.time) params.append('hours', this.currentFilters.time);
        
        const queryString = params.toString();
        return queryString ? `/alerts?${queryString}` : '/alerts';
    }

    // 渲染分页
    renderPagination(total, page, pageSize) {
        this.currentPage = page;
        this.pageSize = pageSize;
        const container = document.getElementById('alertsContainer');
        if (!container) return;

        const totalPages = Math.max(1, Math.ceil(total / pageSize));
        const pagination = document.createElement('nav');
        pagination.className = 'd-flex justify-content-between align-items-center mt-3';
        pagination.innerHTML = `
            <div>共 ${total} 条，${page}/${totalPages} 页</div>
            <div class="btn-group">
                <button class="btn btn-sm btn-outline-secondary" ${page<=1?'disabled':''} id="prevPage">上一页</button>
                <button class="btn btn-sm btn-outline-secondary" ${page>=totalPages?'disabled':''} id="nextPage">下一页</button>
                <select class="form-select form-select-sm ms-2" style="width:auto" id="pageSizeSel">
                    <option ${pageSize==10?'selected':''} value="10">10/页</option>
                    <option ${pageSize==20?'selected':''} value="20">20/页</option>
                    <option ${pageSize==50?'selected':''} value="50">50/页</option>
                    <option ${pageSize==100?'selected':''} value="100">100/页</option>
                </select>
            </div>
        `;
        container.appendChild(pagination);

        pagination.querySelector('#prevPage')?.addEventListener('click', () => {
            this.currentPage = Math.max(1, page-1);
            this.loadAlerts();
        });
        pagination.querySelector('#nextPage')?.addEventListener('click', () => {
            this.currentPage = Math.min(totalPages, page+1);
            this.loadAlerts();
        });
        pagination.querySelector('#pageSizeSel')?.addEventListener('change', (e) => {
            this.pageSize = parseInt(e.target.value);
            this.currentPage = 1;
            this.loadAlerts();
        });
    }

    // 显示告警列表
    displayAlerts(alerts) {
        const container = document.getElementById('alertsContainer');
        if (!container) return;

        if (alerts.length === 0) {
            Loading.empty('alertsContainer', '暂无告警数据', 'inbox');
            return;
        }

        let html = `
            <div class="table-responsive">
                <table class="table table-hover">
                    <thead>
                        <tr>
                            <th>时间</th>
                            <th>规则名称</th>
                            <th>级别</th>
                            <th>消息</th>
                            <th>匹配数</th>
                            <th>操作</th>
                        </tr>
                    </thead>
                    <tbody>
        `;

        alerts.forEach(alert => {
            const levelColor = Utils.getLevelColor(alert.level);
            const messagePreview = Utils.truncateText(alert.message, 100);
            
            html += `
                <tr>
                    <td>${Utils.formatTime(alert.timestamp)}</td>
                    <td>${alert.rule_name}</td>
                    <td><span class="badge bg-${levelColor}">${alert.level}</span></td>
                    <td class="text-truncate" style="max-width: 300px;" title="${alert.message}">
                        ${messagePreview}
                    </td>
                    <td>${alert.count}</td>
                    <td>
                        <button class="btn btn-sm btn-outline-primary" onclick="showAlertDetail('${alert.id}')">
                            <i class="bi bi-eye"></i> 详情
                        </button>
                    </td>
                </tr>
            `;
        });

        html += `
                    </tbody>
                </table>
            </div>
        `;

        container.innerHTML = html;
    }

    // 显示告警详情（从后端获取真实数据）
    async showAlertDetail(alertId) {
        const detailContent = document.getElementById('alertDetailContent');
        if (!detailContent) return;

        try {
            detailContent.innerHTML = '<div class="text-center py-4"><div class="loading-spinner"></div><p class="mt-2 text-muted">加载详情...</p></div>';
            const detail = await API.get(`/alerts/${alertId}`);
            if (!detail || detail.error) {
                throw new Error(detail?.error || '未获取到详情');
            }

            const levelColor = Utils.getLevelColor(detail.level);
            const dataPretty = detail.data ? JSON.stringify(detail.data, null, 2) : '{}';
            const rawMessage = Utils.escapeHTML(detail.message || '');

            detailContent.innerHTML = `
                <div class="row">
                    <div class="col-md-6">
                        <h6>基本信息</h6>
                        <table class="table table-sm">
                            <tr><td>告警ID:</td><td>${detail.id}</td></tr>
                            <tr><td>规则名称:</td><td>${detail.rule_name}</td></tr>
                            <tr><td>级别:</td><td><span class="badge bg-${levelColor}">${detail.level}</span></td></tr>
                            <tr><td>时间:</td><td>${Utils.formatTime(detail.timestamp)}</td></tr>
                            <tr><td>匹配数:</td><td>${detail.count}</td></tr>
                        </table>
                    </div>
                    <div class="col-md-6">
                        <h6>详细信息</h6>
                        <div class="mb-3">
                            <small class="text-muted">原始消息:</small>
                            <div class="border rounded p-2 bg-light" style="max-height: 200px; overflow:auto;">${rawMessage}</div>
                        </div>
                        <h6 class="mt-3">数据</h6>
                        <pre class="bg-light p-3"><code>${dataPretty}</code></pre>
                    </div>
                </div>
            `;

            const modal = new bootstrap.Modal(document.getElementById('alertDetailModal'));
            modal.show();
        } catch (error) {
            detailContent.innerHTML = `<div class="alert alert-danger" role="alert"><i class="bi bi-exclamation-triangle"></i> 加载详情失败: ${error.message}</div>`;
        }
    }

    // 应用筛选器
    applyFilters() {
        this.updateFilters();
        this.loadAlerts();
    }

    // 更新筛选器
    updateFilters() {
        this.currentFilters.rule = document.getElementById('ruleFilter')?.value || '';
        this.currentFilters.level = document.getElementById('levelFilter')?.value || '';
        this.currentFilters.time = document.getElementById('timeFilter')?.value || 24;
    }

    // 刷新告警
    refreshAlerts() {
        this.loadAlerts();
    }

    // 测试通知
    async testNotification() {
        if (!confirm('确定要发送测试通知吗？')) {
            return;
        }
        
        try {
            const response = await API.post('/test/notification');
            if (response.message) {
                Notification.success(response.message);
                // 刷新告警列表
                setTimeout(() => this.loadAlerts(), 1000);
            } else {
                Notification.error(response.error || '测试通知发送失败');
            }
        } catch (error) {
            console.error('测试通知失败:', error);
            Notification.error('测试通知失败: ' + error.message);
        }
    }

    // 开始自动刷新
    startAutoRefresh() {
        this.refreshTimer = setInterval(() => {
            this.loadAlerts();
        }, OpenSearchAlert.config.refreshInterval);
    }

    // 停止自动刷新
    stopAutoRefresh() {
        if (this.refreshTimer) {
            clearInterval(this.refreshTimer);
            this.refreshTimer = null;
        }
    }

    // 销毁
    destroy() {
        this.stopAutoRefresh();
    }
}

// 页面特定的函数
function applyFilters() {
    if (window.alertsPage) {
        window.alertsPage.applyFilters();
    }
}

function refreshAlerts() {
    if (window.alertsPage) {
        window.alertsPage.refreshAlerts();
    }
}

function testNotification() {
    if (window.alertsPage) {
        window.alertsPage.testNotification();
    }
}

function showAlertDetail(alertId) {
    if (window.alertsPage) {
        window.alertsPage.showAlertDetail(alertId);
    }
}

// 页面加载完成后初始化
document.addEventListener('DOMContentLoaded', function() {
    // 检查是否在告警页面
    if (document.getElementById('alertsContainer')) {
        window.alertsPage = new AlertsPage();
    }
});

// 页面卸载时清理
window.addEventListener('beforeunload', function() {
    if (window.alertsPage) {
        window.alertsPage.destroy();
    }
});

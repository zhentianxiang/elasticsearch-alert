/**
 * 规则页面 JavaScript
 */

class RulesPage {
    constructor() {
        this.currentRules = [];
        this.init();
    }

    // 初始化
    init() {
        this.setupEventListeners();
        this.loadRules();
    }

    // 设置事件监听器
    setupEventListeners() {
        // 刷新按钮
        const refreshBtn = document.querySelector('[onclick="refreshRules()"]');
        if (refreshBtn) {
            refreshBtn.addEventListener('click', (e) => {
                e.preventDefault();
                this.loadRules();
            });
        }

        // 添加规则按钮
        const addBtn = document.querySelector('[onclick="showAddRuleModal()"]');
        if (addBtn) {
            addBtn.addEventListener('click', (e) => {
                e.preventDefault();
                this.showAddRuleModal();
            });
        }

        // 保存规则表单
        const saveBtn = document.querySelector('[onclick="saveRule()"]');
        if (saveBtn) {
            saveBtn.addEventListener('click', (e) => {
                e.preventDefault();
                this.saveRule();
            });
        }
    }

    // 加载规则数据
    async loadRules() {
        try {
            Loading.show('rulesContainer', '加载规则数据中...');
            
            const data = await API.get('/rules');
            this.currentRules = data.rules || [];
            
            this.displayRules(this.currentRules);
            this.updateRuleStats(this.currentRules);
        } catch (error) {
            console.error('加载规则失败:', error);
            Loading.error('rulesContainer', '加载规则失败: ' + error.message);
        }
    }

    // 显示规则列表
    displayRules(rules) {
        const container = document.getElementById('rulesContainer');
        if (!container) return;

        if (rules.length === 0) {
            Loading.empty('rulesContainer', '暂无规则数据', 'gear');
            return;
        }

        let html = '<div class="row">';

        // 从后端模板注入的用户角色（若有），或通过 /api/auth/check 缓存
        const currentRole = (window.AuthUser && window.AuthUser.role) || (document.body.dataset.role) || 'viewer';
        const isAdmin = currentRole === 'admin';

        rules.forEach(rule => {
            const statusClass = rule.Enabled ? 'success' : 'danger';
            const statusText = rule.Enabled ? '启用' : '禁用';
            const thresholdVal = rule.Threshold || rule.threshold || rule.ThresholdValue || 0;
            const levelColor = Utils.getLevelColor(rule.Level || 'Low');
            
            html += `
                <div class="col-md-6 col-lg-4 mb-4">
                    <div class="card h-100">
                        <div class="card-header d-flex justify-content-between align-items-center">
                            <h6 class="mb-0">${rule.Name}</h6>
                            <span class="badge bg-${statusClass}">${statusText}</span>
                        </div>
                        <div class="card-body">
                            <div class="mb-2">
                                <small class="text-muted">类型:</small>
                                <span class="badge bg-info">${rule.Type}</span>
                            </div>
                            <div class="mb-2">
                                <small class="text-muted">索引:</small>
                                <code>${rule.Index}</code>
                            </div>
                            <div class="mb-2">
                                <small class="text-muted">阈值:</small>
                                <strong>${thresholdVal}</strong>
                            </div>
                            <div class="mb-2">
                                <small class="text-muted">时间窗口:</small>
                                <strong>${rule.Timeframe}秒</strong>
                            </div>
                            <div class="mb-3">
                                <small class="text-muted">级别:</small>
                                <span class="badge bg-${levelColor}">${rule.Level || '自动'}</span>
                            </div>
                        </div>
                        <div class="card-footer">
                            <div class="btn-group w-100" role="group">
                                ${isAdmin ? `
                                <button class="btn btn-sm btn-outline-warning" onclick="editRule('${rule.Name}')">
                                    <i class="bi bi-pencil"></i> 编辑
                                </button>
                                <button class="btn btn-sm btn-outline-${rule.Enabled ? 'danger' : 'success'}" onclick="toggleRule('${rule.Name}', ${!rule.Enabled})">
                                    <i class="bi bi-${rule.Enabled ? 'pause' : 'play'}"></i> ${rule.Enabled ? '禁用' : '启用'}
                                </button>
                                ` : `
                                <button class="btn btn-sm btn-outline-secondary" disabled>
                                    <i class="bi bi-lock"></i> 只读
                                </button>
                                `}
                            </div>
                        </div>
                    </div>
                </div>
            `;
        });

        html += '</div>';
        container.innerHTML = html;
    }

    // 更新规则统计
    updateRuleStats(rules) {
        const enabledCount = rules.filter(rule => rule.Enabled).length;
        const disabledCount = rules.length - enabledCount;
        const typeCount = new Set(rules.map(rule => rule.Type)).size;
        
        const elements = {
            enabledRules: document.getElementById('enabledRules'),
            disabledRules: document.getElementById('disabledRules'),
            ruleTypes: document.getElementById('ruleTypes')
        };

        if (elements.enabledRules) {
            elements.enabledRules.textContent = enabledCount;
        }
        if (elements.disabledRules) {
            elements.disabledRules.textContent = disabledCount;
        }
        if (elements.ruleTypes) {
            elements.ruleTypes.textContent = typeCount;
        }
    }

    // 显示规则详情
    showRuleDetail(ruleName) {
        const rule = this.currentRules.find(r => r.Name === ruleName);
        if (!rule) return;
        
        const detailContent = document.getElementById('ruleDetailContent');
        if (!detailContent) return;

        detailContent.innerHTML = `
            <div class="row">
                <div class="col-md-6">
                    <h6>基本信息</h6>
                    <table class="table table-sm">
                        <tr><td>规则名称:</td><td>${rule.Name}</td></tr>
                        <tr><td>规则类型:</td><td><span class="badge bg-info">${rule.Type}</span></td></tr>
                        <tr><td>监控索引:</td><td><code>${rule.Index}</code></td></tr>
                        <tr><td>触发阈值:</td><td><strong>${rule.Threshold}</strong></td></tr>
                        <tr><td>时间窗口:</td><td><strong>${rule.Timeframe}秒</strong></td></tr>
                        <tr><td>告警级别:</td><td><span class="badge bg-${Utils.getLevelColor(rule.Level || 'Low')}">${rule.Level || '自动'}</span></td></tr>
                        <tr><td>重复间隔:</td><td><strong>${rule.Realert || 0}分钟</strong></td></tr>
                        <tr><td>状态:</td><td><span class="badge bg-${rule.Enabled ? 'success' : 'danger'}">${rule.Enabled ? '启用' : '禁用'}</span></td></tr>
                    </table>
                </div>
                <div class="col-md-6">
                    <h6>查询条件</h6>
                    <pre class="bg-light p-3"><code>${JSON.stringify(rule.Query, null, 2)}</code></pre>
                </div>
            </div>
            ${rule.AlertText ? `
            <div class="row mt-3">
                <div class="col-12">
                    <h6>告警文本</h6>
                    <p class="text-muted">${rule.AlertText}</p>
                </div>
            </div>
            ` : ''}
        `;
        
        const modal = new bootstrap.Modal(document.getElementById('ruleDetailModal'));
        modal.show();
    }

    // 显示添加规则模态框
    showAddRuleModal() {
        const modal = document.getElementById('ruleEditModal');
        if (!modal) return;

        document.getElementById('ruleEditTitle').textContent = '添加规则';
        document.getElementById('ruleEditForm').reset();
        document.getElementById('ruleEnabled').checked = true;
        
        const bootstrapModal = new bootstrap.Modal(modal);
        bootstrapModal.show();
    }

    // 编辑规则
    editRule(ruleName) {
        const rule = this.currentRules.find(r => r.Name === ruleName);
        if (!rule) return;
        
        const modal = document.getElementById('ruleEditModal');
        if (!modal) return;

        document.getElementById('ruleEditTitle').textContent = '编辑规则';
        document.getElementById('ruleName').value = rule.Name;
        document.getElementById('ruleType').value = rule.Type;
        document.getElementById('ruleIndex').value = rule.Index;
        document.getElementById('ruleThreshold').value = rule.Threshold;
        document.getElementById('ruleTimeframe').value = rule.Timeframe;
        document.getElementById('ruleLevel').value = rule.Level || '';
        document.getElementById('ruleEnabled').checked = rule.Enabled;
        document.getElementById('ruleRealert').value = rule.Realert || 0;
        document.getElementById('ruleAlertText').value = rule.AlertText || '';
        document.getElementById('ruleQuery').value = JSON.stringify(rule.Query, null, 2);

        // 新增字段填充
        const alertList = (rule.Alert || []).map(x => (typeof x === 'string' ? x.toLowerCase() : x));
        // 若规则未显式指定 Alert，则默认全选
        const allChannels = ['email','dingtalk','wechat','feishu'];
        document.getElementById('alertEmail').checked = alertList.length === 0 ? true : alertList.includes('email');
        document.getElementById('alertDingTalk').checked = alertList.length === 0 ? true : alertList.includes('dingtalk');
        document.getElementById('alertWeChat').checked = alertList.length === 0 ? true : alertList.includes('wechat');
        document.getElementById('alertFeishu').checked = alertList.length === 0 ? true : alertList.includes('feishu');

        document.getElementById('ruleAlertTextArgs').value = (rule.AlertTextArgs || []).join(', ');
        document.getElementById('ruleQueryKey').value = (rule.QueryKey || []).join(', ');
        
        const bootstrapModal = new bootstrap.Modal(modal);
        bootstrapModal.show();
    }

    // 保存规则
    async saveRule() {
        try {
            const rule = {
                Name: document.getElementById('ruleName').value,
                Type: document.getElementById('ruleType').value,
                Index: document.getElementById('ruleIndex').value,
                Threshold: parseInt(document.getElementById('ruleThreshold').value),
                Timeframe: parseInt(document.getElementById('ruleTimeframe').value),
                Level: document.getElementById('ruleLevel').value || '',
                Enabled: document.getElementById('ruleEnabled').checked,
                Realert: parseInt(document.getElementById('ruleRealert').value) || 0,
                AlertText: document.getElementById('ruleAlertText').value || '',
                Query: JSON.parse(document.getElementById('ruleQuery').value || '{}')
            };

            // 采集通知渠道
            const alerts = [];
            if (document.getElementById('alertEmail').checked) alerts.push('email');
            if (document.getElementById('alertDingTalk').checked) alerts.push('dingtalk');
            if (document.getElementById('alertWeChat').checked) alerts.push('wechat');
            if (document.getElementById('alertFeishu').checked) alerts.push('feishu');
            if (alerts.length > 0) rule.Alert = alerts;

            // 采集模板参数与去重键
            const textArgs = (document.getElementById('ruleAlertTextArgs').value || '').split(',').map(s => s.trim()).filter(Boolean);
            if (textArgs.length > 0) rule.AlertTextArgs = textArgs;
            let queryKey = (document.getElementById('ruleQueryKey').value || '').split(',').map(s => s.trim()).filter(Boolean);
            if (queryKey.length === 0) {
                // 默认 Query Key
                queryKey = ['@timestamp','involvedObject.name'];
            }
            rule.QueryKey = queryKey;

            const resp = await API.post('/rules', rule);
            if (resp && !resp.error) {
                Notification.success('规则保存成功！');
                const modal = bootstrap.Modal.getInstance(document.getElementById('ruleEditModal'));
                modal.hide();
                this.loadRules();
            } else {
                throw new Error(resp?.error || '保存失败');
            }
        } catch (error) {
            console.error('保存规则失败:', error);
            Notification.error('保存规则失败: ' + error.message);
        }
    }

    // 切换规则状态
    async toggleRule(ruleName, enabled) {
        if (!confirm(`确定要${enabled ? '启用' : '禁用'}规则 "${ruleName}" 吗？`)) {
            return;
        }
        
        try {
            const endpoint = enabled ? `/rules/${encodeURIComponent(ruleName)}/enable` : `/rules/${encodeURIComponent(ruleName)}/disable`;
            const resp = await API.post(endpoint, {});
            if (resp && !resp.error) {
                Notification.success(`规则 "${ruleName}" 已${enabled ? '启用' : '禁用'}！`);
                // 立即刷新
                this.loadRules();
            } else {
                throw new Error(resp?.error || '操作失败');
            }
        } catch (error) {
            console.error('切换规则状态失败:', error);
            Notification.error('切换规则状态失败: ' + error.message);
        }
    }

    // 刷新规则
    refreshRules() {
        this.loadRules();
    }
}

// 页面特定的函数
function refreshRules() {
    if (window.rulesPage) {
        window.rulesPage.refreshRules();
    }
}

function showAddRuleModal() {
    if (window.rulesPage) {
        window.rulesPage.showAddRuleModal();
    }
}

function editRule(ruleName) {
    if (window.rulesPage) {
        window.rulesPage.editRule(ruleName);
    }
}

function saveRule() {
    if (window.rulesPage) {
        window.rulesPage.saveRule();
    }
}

function toggleRule(ruleName, enabled) {
    if (window.rulesPage) {
        window.rulesPage.toggleRule(ruleName, enabled);
    }
}

// 详情功能已移除

// 页面加载完成后初始化
document.addEventListener('DOMContentLoaded', function() {
    // 检查是否在规则页面
    if (document.getElementById('rulesContainer')) {
        window.rulesPage = new RulesPage();
    }
});

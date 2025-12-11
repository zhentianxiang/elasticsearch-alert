/**
 * Dashboard 页面 JavaScript
 */

class Dashboard {
    constructor() {
        this.charts = {};
        this.currentTimeRange = 24;
        this.refreshTimer = null;
        this.init();
    }

    // 初始化
    init() {
        this.initCharts();
        this.loadData();
        this.setupEventListeners();
        this.startAutoRefresh();
    }

    // 初始化图表
    initCharts() {
        this.initLevelChart();
        this.initTrendChart();
    }

    // 初始化告警级别分布饼图
    initLevelChart() {
        const ctx = document.getElementById('levelChart');
        if (!ctx) return;

        this.charts.levelChart = ChartManager.createPieChart('levelChart', {
            labels: ['Critical', 'High', 'Medium', 'Low', 'Info'],
            values: [0, 0, 0, 0, 0],
            colors: [
                OpenSearchAlert.config.chartColors.critical,
                OpenSearchAlert.config.chartColors.high,
                OpenSearchAlert.config.chartColors.medium,
                OpenSearchAlert.config.chartColors.low,
                OpenSearchAlert.config.chartColors.info
            ]
        });
    }

    // 初始化告警时间趋势图
    initTrendChart() {
        const ctx = document.getElementById('trendChart');
        if (!ctx) return;

        this.charts.trendChart = ChartManager.createLineChart('trendChart', {
            labels: this.generateHourLabels(),
            values: new Array(24).fill(0),
            label: '告警数量'
        });
    }

    // 生成24小时标签
    generateHourLabels() {
        const labels = [];
        for (let i = 0; i < 24; i++) {
            labels.push(`${i.toString().padStart(2, '0')}:00`);
        }
        return labels;
    }

    // 设置事件监听器
    setupEventListeners() {
        // 时间范围选择
        const timeRangeDropdown = document.querySelector('.dropdown-menu');
        if (timeRangeDropdown) {
            timeRangeDropdown.addEventListener('click', (e) => {
                const link = e.target.closest('a');
                if (link) {
                    e.preventDefault();
                    const hours = parseInt(link.dataset.hours);
                    if (hours) {
                        this.updateTimeRange(hours);
                    }
                }
            });
        }

        // 刷新按钮
        const refreshBtn = document.querySelector('[onclick="refreshData()"]');
        if (refreshBtn) {
            refreshBtn.addEventListener('click', (e) => {
                e.preventDefault();
                this.loadData();
            });
        }
    }

    // 加载数据
    async loadData() {
        try {
            Loading.show('dashboardContent', '加载数据中...');
            // 并行获取统计与规则
            const [statsData, rules] = await Promise.all([
                this.loadAlertStats(),
                this.loadRules()
            ]);

            const activeRulesCount = rules.filter(r => r.Enabled).length;

            this.updateStatsCards({ ...statsData, active_rules: activeRulesCount });
            this.updateLevelChart(statsData.level_stats);
            this.updateTrendChart(statsData.hourly_stats);
            this.updateRecentAlerts(statsData.recent_alerts);

            Loading.hide('dashboardContent');
        } catch (error) {
            console.error('加载数据失败:', error);
            Loading.error('dashboardContent', '加载数据失败: ' + error.message);
        }
    }

    // 加载告警统计
    async loadAlertStats() {
        return await API.get(`/alerts/stats?hours=${this.currentTimeRange}`);
    }

    // 加载规则列表
    async loadRules() {
        const resp = await API.get('/rules');
        return resp?.rules || [];
    }


    // 更新统计卡片
    updateStatsCards(data) {
        const elements = {
            totalAlerts: document.getElementById('totalAlerts'),
            criticalAlerts: document.getElementById('criticalAlerts'),
            highAlerts: document.getElementById('highAlerts'),
            activeRules: document.getElementById('activeRules')
        };

        if (elements.totalAlerts) {
            elements.totalAlerts.textContent = data.total_alerts || 0;
        }
        if (elements.criticalAlerts) {
            elements.criticalAlerts.textContent = data.level_stats?.Critical || 0;
        }
        if (elements.highAlerts) {
            elements.highAlerts.textContent = data.level_stats?.High || 0;
        }
        if (elements.activeRules) {
            elements.activeRules.textContent = data.active_rules || 0;
        }
    }

    // 更新级别分布图
    updateLevelChart(levelStats) {
        if (!this.charts.levelChart || !levelStats) return;

        const data = {
            labels: ['Critical', 'High', 'Medium', 'Low', 'Info'],
            values: [
                levelStats.Critical || 0,
                levelStats.High || 0,
                levelStats.Medium || 0,
                levelStats.Low || 0,
                levelStats.Info || 0
            ]
        };

        ChartManager.updateChart(this.charts.levelChart, data);
    }

    // 更新时间趋势图
    updateTrendChart(hourlyStats) {
        if (!this.charts.trendChart || !hourlyStats) return;

        const data = {
            labels: this.generateHourLabels(),
            values: new Array(24).fill(0)
        };

        // 填充实际数据
        hourlyStats.forEach(stat => {
            if (stat.hour >= 0 && stat.hour < 24) {
                data.values[stat.hour] = stat.count;
            }
        });

        ChartManager.updateChart(this.charts.trendChart, data);
    }

    // 更新最近告警列表
    updateRecentAlerts(alerts) {
        const tbody = document.querySelector('#recentAlertsTable tbody');
        if (!tbody) return;

        if (!alerts || alerts.length === 0) {
            tbody.innerHTML = '<tr><td colspan="5" class="text-center text-muted">暂无告警数据</td></tr>';
            return;
        }

        tbody.innerHTML = '';
        alerts.forEach(alert => {
            const row = document.createElement('tr');
            row.innerHTML = `
                <td>${Utils.formatTime(alert.timestamp)}</td>
                <td>${alert.rule_name}</td>
                <td><span class="badge bg-${Utils.getLevelColor(alert.level)}">${alert.level}</span></td>
                <td class="text-truncate" style="max-width: 300px;" title="${alert.message}">${Utils.truncateText(alert.message, 50)}</td>
                <td>${alert.count}</td>
            `;
            tbody.appendChild(row);
        });
    }


    // 更新时间范围
    updateTimeRange(hours) {
        this.currentTimeRange = hours;
        this.loadData();
        
        // 更新下拉菜单显示
        const dropdownToggle = document.querySelector('.dropdown-toggle');
        if (dropdownToggle) {
            const timeText = this.getTimeRangeText(hours);
            dropdownToggle.textContent = timeText;
        }
    }

    // 获取时间范围文本
    getTimeRangeText(hours) {
        const timeRanges = {
            1: '最近1小时',
            6: '最近6小时',
            24: '最近24小时',
            168: '最近7天'
        };
        return timeRanges[hours] || '最近24小时';
    }

    // 开始自动刷新
    startAutoRefresh() {
        this.refreshTimer = setInterval(() => {
            this.loadData();
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
        Object.values(this.charts).forEach(chart => {
            ChartManager.destroy(chart);
        });
    }
}

// 页面特定的函数
function refreshData() {
    if (window.dashboard) {
        window.dashboard.loadData();
    }
}

function updateTimeRange(hours) {
    if (window.dashboard) {
        window.dashboard.updateTimeRange(hours);
    }
}

// 页面加载完成后初始化
document.addEventListener('DOMContentLoaded', function() {
    // 检查是否在 Dashboard 页面
    if (document.getElementById('levelChart') || document.getElementById('trendChart')) {
        window.dashboard = new Dashboard();
    }
});

// 页面卸载时清理
window.addEventListener('beforeunload', function() {
    if (window.dashboard) {
        window.dashboard.destroy();
    }
});

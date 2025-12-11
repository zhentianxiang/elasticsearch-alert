/**
 * OpenSearch 告警系统 - 公共 JavaScript 函数
 */

// 全局变量
window.OpenSearchAlert = {
    config: {
        apiBase: '/api',
        refreshInterval: 30000, // 30秒
        chartColors: {
            critical: '#ef4444',
            high: '#f59e0b',
            medium: '#06b6d4',
            low: '#6c757d',
            info: '#3b82f6'
        }
    },
    charts: {},
    timers: {}
};

/**
 * 工具函数
 */
const Utils = {
    // 格式化时间
    formatTime(timestamp) {
        if (!timestamp) return '';
        const date = new Date(timestamp);
        return date.toLocaleString('zh-CN', {
            year: 'numeric',
            month: '2-digit',
            day: '2-digit',
            hour: '2-digit',
            minute: '2-digit',
            second: '2-digit'
        });
    },

    // 格式化相对时间
    formatRelativeTime(timestamp) {
        if (!timestamp) return '';
        const now = new Date();
        const date = new Date(timestamp);
        const diff = now - date;
        
        const minutes = Math.floor(diff / 60000);
        const hours = Math.floor(diff / 3600000);
        const days = Math.floor(diff / 86400000);
        
        if (minutes < 1) return '刚刚';
        if (minutes < 60) return `${minutes}分钟前`;
        if (hours < 24) return `${hours}小时前`;
        return `${days}天前`;
    },

    // 获取告警级别颜色
    getLevelColor(level) {
        const colors = {
            'Critical': 'danger',
            'High': 'warning',
            'Medium': 'info',
            'Low': 'secondary',
            'Info': 'primary'
        };
        return colors[level] || 'secondary';
    },

    // 获取告警级别样式类
    getLevelClass(level) {
        return 'level-' + level.toLowerCase();
    },

    // 截断文本
    truncateText(text, maxLength = 100) {
        if (!text) return '';
        if (text.length <= maxLength) return text;
        return text.substring(0, maxLength) + '...';
    },

    // 转义HTML以防止XSS
    escapeHTML(text) {
        if (text === undefined || text === null) return '';
        return String(text)
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;')
            .replace(/'/g, '&#039;');
    },

    // 防抖函数
    debounce(func, wait) {
        let timeout;
        return function executedFunction(...args) {
            const later = () => {
                clearTimeout(timeout);
                func(...args);
            };
            clearTimeout(timeout);
            timeout = setTimeout(later, wait);
        };
    },

    // 节流函数
    throttle(func, limit) {
        let inThrottle;
        return function() {
            const args = arguments;
            const context = this;
            if (!inThrottle) {
                func.apply(context, args);
                inThrottle = true;
                setTimeout(() => inThrottle = false, limit);
            }
        };
    }
};

/**
 * API 请求函数
 */
const API = {
    // 基础请求函数
    async request(url, options = {}) {
        const defaultOptions = {
            headers: {
                'Content-Type': 'application/json',
            },
            credentials: 'same-origin', // 确保发送cookies
        };

        const config = { ...defaultOptions, ...options };
        
        try {
            const response = await fetch(url, config);
            
            if (!response.ok) {
                if (response.status === 401) {
                    // 401 未授权，只有在非登录页面时才重定向
                    const currentPath = window.location.pathname;
                    if (currentPath !== '/login') {
                        window.location.href = '/login';
                    }
                    return;
                }
                const data = await response.json().catch(() => ({}));
                throw new Error(data.error || `HTTP ${response.status}`);
            }
            
            const data = await response.json();
            return data;
        } catch (error) {
            console.error('API 请求失败:', error);
            throw error;
        }
    },

    // GET 请求
    async get(endpoint) {
        return this.request(`${OpenSearchAlert.config.apiBase}${endpoint}`);
    },

    // POST 请求
    async post(endpoint, data) {
        return this.request(`${OpenSearchAlert.config.apiBase}${endpoint}`, {
            method: 'POST',
            body: JSON.stringify(data)
        });
    },

    // PUT 请求
    async put(endpoint, data) {
        return this.request(`${OpenSearchAlert.config.apiBase}${endpoint}`, {
            method: 'PUT',
            body: JSON.stringify(data)
        });
    },

    // DELETE 请求
    async delete(endpoint) {
        return this.request(`${OpenSearchAlert.config.apiBase}${endpoint}`, {
            method: 'DELETE'
        });
    }
};

/**
 * 通知管理
 */
const Notification = {
    // 显示成功消息
    success(message, duration = 3000) {
        this.show(message, 'success', duration);
    },

    // 显示错误消息
    error(message, duration = 5000) {
        this.show(message, 'danger', duration);
    },

    // 显示警告消息
    warning(message, duration = 4000) {
        this.show(message, 'warning', duration);
    },

    // 显示信息消息
    info(message, duration = 3000) {
        this.show(message, 'info', duration);
    },

    // 显示通知
    show(message, type = 'info', duration = 3000) {
        const alert = document.createElement('div');
        alert.className = `alert alert-${type} alert-dismissible fade show position-fixed`;
        alert.style.cssText = 'top: 20px; right: 20px; z-index: 9999; min-width: 300px; max-width: 500px;';
        alert.innerHTML = `
            <i class="bi bi-${this.getIcon(type)}"></i> ${message}
            <button type="button" class="btn-close" data-bs-dismiss="alert"></button>
        `;
        
        document.body.appendChild(alert);
        
        // 自动移除
        setTimeout(() => {
            if (alert.parentNode) {
                alert.parentNode.removeChild(alert);
            }
        }, duration);
    },

    // 获取图标
    getIcon(type) {
        const icons = {
            success: 'check-circle',
            danger: 'exclamation-triangle',
            warning: 'exclamation-circle',
            info: 'info-circle'
        };
        return icons[type] || 'info-circle';
    }
};

/**
 * 加载状态管理
 */
const Loading = {
    // 显示加载状态
    show(elementId, message = '加载中...') {
        const element = document.getElementById(elementId);
        if (element) {
            element.innerHTML = `
                <div class="text-center py-4">
                    <div class="loading-spinner"></div>
                    <p class="mt-2 text-muted">${message}</p>
                </div>
            `;
        }
    },

    // 隐藏加载状态
    hide(elementId) {
        const element = document.getElementById(elementId);
        if (element) {
            element.innerHTML = '';
        }
    },

    // 显示错误状态
    error(elementId, message) {
        const element = document.getElementById(elementId);
        if (element) {
            element.innerHTML = `
                <div class="alert alert-danger" role="alert">
                    <i class="bi bi-exclamation-triangle"></i> ${message}
                </div>
            `;
        }
    },

    // 显示空状态
    empty(elementId, message = '暂无数据', icon = 'inbox') {
        const element = document.getElementById(elementId);
        if (element) {
            element.innerHTML = `
                <div class="empty-state">
                    <i class="bi bi-${icon}"></i>
                    <p>${message}</p>
                </div>
            `;
        }
    }
};

/**
 * 图表管理
 */
const ChartManager = {
    // 创建饼图
    createPieChart(canvasId, data, options = {}) {
        const ctx = document.getElementById(canvasId);
        if (!ctx) return null;

        const defaultOptions = {
            type: 'doughnut',
            data: {
                labels: data.labels || [],
                datasets: [{
                    data: data.values || [],
                    backgroundColor: data.colors || Object.values(OpenSearchAlert.config.chartColors),
                    borderWidth: 2,
                    borderColor: '#fff'
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: {
                        position: 'bottom',
                        labels: {
                            padding: 20,
                            usePointStyle: true
                        }
                    },
                    tooltip: {
                        callbacks: {
                            label: function(context) {
                                const total = context.dataset.data.reduce((a, b) => a + b, 0);
                                const percentage = ((context.parsed / total) * 100).toFixed(1);
                                return `${context.label}: ${context.parsed} (${percentage}%)`;
                            }
                        }
                    }
                }
            }
        };

        const config = this.mergeOptions(defaultOptions, options);
        return new Chart(ctx, config);
    },

    // 创建折线图
    createLineChart(canvasId, data, options = {}) {
        const ctx = document.getElementById(canvasId);
        if (!ctx) return null;

        const defaultOptions = {
            type: 'line',
            data: {
                labels: data.labels || [],
                datasets: [{
                    label: data.label || '数据',
                    data: data.values || [],
                    borderColor: OpenSearchAlert.config.chartColors.info,
                    backgroundColor: 'rgba(59, 130, 246, 0.1)',
                    tension: 0.4,
                    fill: true,
                    pointBackgroundColor: OpenSearchAlert.config.chartColors.info,
                    pointBorderColor: '#fff',
                    pointBorderWidth: 2,
                    pointRadius: 4,
                    pointHoverRadius: 6
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                scales: {
                    y: {
                        beginAtZero: true,
                        ticks: {
                            stepSize: 1
                        },
                        grid: {
                            color: 'rgba(0, 0, 0, 0.1)'
                        }
                    },
                    x: {
                        grid: {
                            color: 'rgba(0, 0, 0, 0.1)'
                        }
                    }
                },
                plugins: {
                    legend: {
                        display: false
                    },
                    tooltip: {
                        mode: 'index',
                        intersect: false
                    }
                },
                interaction: {
                    mode: 'nearest',
                    axis: 'x',
                    intersect: false
                }
            }
        };

        const config = this.mergeOptions(defaultOptions, options);
        return new Chart(ctx, config);
    },

    // 创建柱状图
    createBarChart(canvasId, data, options = {}) {
        const ctx = document.getElementById(canvasId);
        if (!ctx) return null;

        const defaultOptions = {
            type: 'bar',
            data: {
                labels: data.labels || [],
                datasets: [{
                    label: data.label || '数据',
                    data: data.values || [],
                    backgroundColor: OpenSearchAlert.config.chartColors.info,
                    borderColor: OpenSearchAlert.config.chartColors.info,
                    borderWidth: 1
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                scales: {
                    y: {
                        beginAtZero: true,
                        ticks: {
                            stepSize: 1
                        }
                    }
                },
                plugins: {
                    legend: {
                        display: false
                    }
                }
            }
        };

        const config = this.mergeOptions(defaultOptions, options);
        return new Chart(ctx, config);
    },

    // 更新图表数据
    updateChart(chart, data) {
        if (!chart) return;
        
        // 更新标签
        chart.data.labels = data.labels || chart.data.labels;

        // 支持两种传参：
        // 1) 一维数组 values: [1,2,3] -> 更新第一个数据集
        // 2) 按数据集索引的二维数组 values: [[...], [...]]
        if (Array.isArray(data.values)) {
            const valuesIs2D = Array.isArray(data.values[0]);

            if (!valuesIs2D) {
                // 单数据集情况
                if (chart.data.datasets[0]) {
                    chart.data.datasets[0].data = data.values;
                }
            } else {
                // 多数据集情况
                chart.data.datasets.forEach((dataset, index) => {
                    if (Array.isArray(data.values[index])) {
                        dataset.data = data.values[index];
                    }
                });
            }
        }

        chart.update();
    },

    // 合并选项
    mergeOptions(defaultOptions, customOptions) {
        return JSON.parse(JSON.stringify(defaultOptions, (key, value) => {
            if (customOptions[key] !== undefined) {
                return customOptions[key];
            }
            return value;
        }));
    },

    // 销毁图表
    destroy(chart) {
        if (chart) {
            chart.destroy();
        }
    }
};

/**
 * 认证管理
 */
const Auth = {
    // 检查认证状态
    async checkAuth() {
        try {
            const response = await API.get('/auth/check');
            return response && response.authenticated === true;
        } catch (error) {
            console.log('认证检查失败:', error);
            return false;
        }
    },

    // 登录
    async login(username, password) {
        try {
            const response = await API.post('/login', { username, password });
            if (response.success) {
                Notification.success('登录成功');
                return response.user;
            } else {
                Notification.error(response.message || '登录失败');
                return null;
            }
        } catch (error) {
            Notification.error('登录请求失败: ' + error.message);
            return null;
        }
    },

    // 登出
    async logout() {
        try {
            await API.post('/logout');
            Notification.info('已登出');
            window.location.href = '/login';
        } catch (error) {
            console.error('登出失败:', error);
            window.location.href = '/login';
        }
    }
};

/**
 * 页面初始化
 */
const PageInit = {
    // 初始化页面
    init() {
        this.setupEventListeners();
        
        // 检查认证状态（仅在非登录页面）
        const currentPath = window.location.pathname;
        if (currentPath !== '/login') {
            this.checkAuthAndRedirect();
        }
        
        this.setupAutoRefresh();
    },

    // 设置事件监听器
    setupEventListeners() {
        // 登出按钮
        const logoutBtn = document.querySelector('[onclick="logout()"]');
        if (logoutBtn) {
            logoutBtn.addEventListener('click', (e) => {
                e.preventDefault();
                Auth.logout();
            });
        }

        // 刷新按钮
        const refreshBtn = document.querySelector('[onclick="refreshData()"]');
        if (refreshBtn) {
            refreshBtn.addEventListener('click', (e) => {
                e.preventDefault();
                this.refreshData();
            });
        }

        // 表单提交
        const forms = document.querySelectorAll('form');
        forms.forEach(form => {
            form.addEventListener('submit', (e) => {
                e.preventDefault();
                this.handleFormSubmit(form);
            });
        });
    },

    // 检查认证并重定向
    async checkAuthAndRedirect() {
        try {
            const isAuthenticated = await Auth.checkAuth();
            const currentPath = window.location.pathname;
            
            if (!isAuthenticated && currentPath !== '/login') {
                window.location.href = '/login';
            } else if (isAuthenticated && currentPath === '/login') {
                window.location.href = '/dashboard';
            }
        } catch (error) {
            console.log('认证检查失败，跳过重定向:', error);
            // 在登录页面时，认证失败不应该重定向
        }
    },

    // 设置自动刷新
    setupAutoRefresh() {
        // 只在非登录页面设置自动刷新
        const currentPath = window.location.pathname;
        if (currentPath !== '/login') {
            // 每30秒检查一次认证状态
            setInterval(() => {
                this.checkAuthAndRedirect();
            }, 30000);
        }
    },

    // 刷新数据
    refreshData() {
        window.location.reload();
    },

    // 处理表单提交
    handleFormSubmit(form) {
        const formData = new FormData(form);
        const data = Object.fromEntries(formData);
        
        // 这里可以根据表单ID或类名来处理不同的表单
        console.log('表单提交:', data);
    }
};

/**
 * 页面加载完成后初始化
 */
document.addEventListener('DOMContentLoaded', function() {
    PageInit.init();
});

// 导出到全局作用域
window.Utils = Utils;
window.API = API;
window.Notification = Notification;
window.Loading = Loading;
window.ChartManager = ChartManager;
window.Auth = Auth;

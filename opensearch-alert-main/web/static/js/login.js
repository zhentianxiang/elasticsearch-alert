/**
 * 登录页面 JavaScript
 */

class LoginPage {
    constructor() {
        this.init();
    }

    // 初始化
    init() {
        this.setupEventListeners();
        this.checkExistingAuth();
    }

    // 设置事件监听器
    setupEventListeners() {
        const loginForm = document.getElementById('loginForm');
        if (loginForm) {
            loginForm.addEventListener('submit', (e) => {
                e.preventDefault();
                this.handleLogin();
            });
        }

        // 回车键登录
        document.addEventListener('keypress', (e) => {
            if (e.key === 'Enter') {
                this.handleLogin();
            }
        });
    }

    // 检查现有认证状态
    async checkExistingAuth() {
        try {
            const isAuthenticated = await Auth.checkAuth();
            if (isAuthenticated) {
                console.log('用户已认证，跳转到 Dashboard');
                window.location.href = '/dashboard';
            }
        } catch (error) {
            console.log('认证检查失败，继续显示登录页面:', error);
        }
    }

    // 处理登录
    async handleLogin() {
        const username = document.getElementById('username')?.value;
        const password = document.getElementById('password')?.value;
        const loginBtn = document.getElementById('loginBtn');
        const alertContainer = document.getElementById('alertContainer');
        
        if (!username || !password) {
            this.showAlert('danger', '请输入用户名和密码');
            return;
        }
        
        // 显示加载状态
        this.setLoadingState(loginBtn, true);
        this.clearAlerts(alertContainer);
        
        try {
            const user = await Auth.login(username, password);
            if (user) {
                // 登录成功，跳转到 Dashboard
                window.location.href = '/dashboard';
            }
        } catch (error) {
            console.error('登录失败:', error);
            this.showAlert('danger', '登录失败: ' + error.message);
        } finally {
            // 恢复按钮状态
            this.setLoadingState(loginBtn, false);
        }
    }

    // 设置加载状态
    setLoadingState(button, loading) {
        if (!button) return;
        
        if (loading) {
            button.disabled = true;
            button.innerHTML = '<span class="spinner-border spinner-border-sm me-2"></span>登录中...';
        } else {
            button.disabled = false;
            button.innerHTML = '<i class="bi bi-box-arrow-in-right"></i> 登录';
        }
    }

    // 显示警告信息
    showAlert(type, message) {
        const alertContainer = document.getElementById('alertContainer');
        if (!alertContainer) return;

        const alert = document.createElement('div');
        alert.className = `alert alert-${type} alert-dismissible fade show`;
        alert.innerHTML = `
            <i class="bi bi-exclamation-triangle"></i> ${message}
            <button type="button" class="btn-close" data-bs-dismiss="alert"></button>
        `;
        
        alertContainer.appendChild(alert);
    }

    // 清除警告信息
    clearAlerts(container) {
        if (container) {
            container.innerHTML = '';
        }
    }
}

// 页面加载完成后初始化
document.addEventListener('DOMContentLoaded', function() {
    // 检查是否在登录页面
    if (document.getElementById('loginForm')) {
        window.loginPage = new LoginPage();
    }
});

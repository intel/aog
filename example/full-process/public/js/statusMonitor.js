/**
 * AOG智能内容创作助手 - 状态监控模块
 * Copyright 2024-2025 Intel Corporation
 */

class StatusMonitor {
    constructor() {
        this.services = ['chat', 'text-to-image', 'speech-to-text', 'text-to-speech'];
        this.serviceStatus = new Map();
        this.performanceMetrics = {
            totalResponseTime: 0,
            localRequests: 0,
            remoteRequests: 0,
            requestCount: 0
        };
        
        this.monitoringInterval = null;
        this.updateInterval = 5000; // 5秒更新一次
        this.isCollapsed = false;
        
        this.callbacks = {
            onStatusChange: null,
            onMetricsUpdate: null
        };
        
        this.initializeElements();
        this.initializeServices();
    }

    /**
     * 初始化DOM元素
     */
    initializeElements() {
        this.statusPanel = document.getElementById('status-panel');
        this.panelToggle = document.getElementById('panel-toggle');
        this.panelContent = document.getElementById('panel-content');
        this.serviceStatusContainer = document.getElementById('service-status');
        this.performanceContainer = document.getElementById('performance-metrics');
        this.aogStatus = document.getElementById('aog-status');
        
        this.bindEvents();
    }

    /**
     * 绑定事件
     */
    bindEvents() {
        if (this.panelToggle) {
            this.panelToggle.addEventListener('click', () => this.togglePanel());
        }
    }

    /**
     * 初始化服务状态
     */
    initializeServices() {
        this.services.forEach(service => {
            this.serviceStatus.set(service, {
                status: 'unknown',
                lastCheck: null,
                responseTime: 0,
                errorCount: 0
            });
        });
        
        this.updateServiceStatusUI();
    }

    /**
     * 设置回调函数
     * @param {Object} callbacks - 回调函数对象
     */
    setCallbacks(callbacks) {
        this.callbacks = { ...this.callbacks, ...callbacks };
    }

    /**
     * 开始监控
     */
    startMonitoring() {
        console.log('[StatusMonitor] Starting service monitoring...');
        
        // 立即检查一次
        this.checkAllServices();
        
        // 设置定期检查
        this.monitoringInterval = setInterval(() => {
            this.checkAllServices();
        }, this.updateInterval);
        
        this.updateAOGStatus('已连接', 'connected');
    }

    /**
     * 停止监控
     */
    stopMonitoring() {
        console.log('[StatusMonitor] Stopping service monitoring...');
        
        if (this.monitoringInterval) {
            clearInterval(this.monitoringInterval);
            this.monitoringInterval = null;
        }
        
        this.updateAOGStatus('已断开', 'disconnected');
    }

    /**
     * 检查所有服务状态
     */
    async checkAllServices() {
        try {
            const response = await fetch('/api/status');
            
            if (response.ok) {
                const data = await response.json();
                this.updateServicesStatus(data.services);
            } else {
                this.handleStatusCheckError('服务状态检查失败');
            }
        } catch (error) {
            console.error('[StatusMonitor] Status check failed:', error);
            this.handleStatusCheckError('无法连接到服务');
        }
    }

    /**
     * 更新服务状态
     * @param {Object} servicesData - 服务状态数据
     */
    updateServicesStatus(servicesData) {
        const now = Date.now();
        
        this.services.forEach(service => {
            const status = servicesData[service] || 'unknown';
            const serviceInfo = this.serviceStatus.get(service);
            
            if (serviceInfo) {
                serviceInfo.status = status;
                serviceInfo.lastCheck = now;
                
                if (status !== 'available') {
                    serviceInfo.errorCount++;
                }
            }
        });
        
        this.updateServiceStatusUI();
        
        if (this.callbacks.onStatusChange) {
            this.callbacks.onStatusChange(servicesData);
        }
    }

    /**
     * 处理状态检查错误
     * @param {string} message - 错误消息
     */
    handleStatusCheckError(message) {
        console.error('[StatusMonitor] Status check error:', message);
        
        // 将所有服务标记为错误状态
        this.services.forEach(service => {
            const serviceInfo = this.serviceStatus.get(service);
            if (serviceInfo) {
                serviceInfo.status = 'error';
                serviceInfo.errorCount++;
            }
        });
        
        this.updateServiceStatusUI();
        this.updateAOGStatus(message, 'error');
    }

    /**
     * 更新服务状态UI
     */
    updateServiceStatusUI() {
        if (!this.serviceStatusContainer) return;
        
        const serviceItems = this.serviceStatusContainer.querySelectorAll('.service-item');
        
        serviceItems.forEach(item => {
            const indicator = item.querySelector('.service-indicator');
            const serviceName = indicator?.getAttribute('data-service');
            
            if (serviceName && indicator) {
                const serviceInfo = this.serviceStatus.get(serviceName);
                
                if (serviceInfo) {
                    const status = this.mapStatusToHealth(serviceInfo.status);
                    indicator.setAttribute('data-status', status);
                    
                    // 更新提示信息
                    const title = `${serviceName}: ${serviceInfo.status} (错误次数: ${serviceInfo.errorCount})`;
                    indicator.setAttribute('title', title);
                }
            }
        });
    }

    /**
     * 映射状态到健康状态
     * @param {string} status - 原始状态
     * @returns {string} 健康状态
     */
    mapStatusToHealth(status) {
        switch (status) {
            case 'available':
                return 'healthy';
            case 'unavailable':
            case 'error':
                return 'error';
            default:
                return 'warning';
        }
    }

    /**
     * 记录服务调用
     * @param {string} service - 服务名称
     * @param {number} responseTime - 响应时间
     * @param {string} provider - 服务提供商 (local/remote)
     */
    recordServiceCall(service, responseTime, provider = 'unknown') {
        console.log(`[StatusMonitor] Recording call: ${service}, ${responseTime}ms, ${provider}`);
        
        // 更新性能指标
        this.performanceMetrics.requestCount++;
        this.performanceMetrics.totalResponseTime += responseTime;
        
        if (provider === 'local') {
            this.performanceMetrics.localRequests++;
        } else if (provider === 'remote') {
            this.performanceMetrics.remoteRequests++;
        }
        
        // 更新服务状态
        const serviceInfo = this.serviceStatus.get(service);
        if (serviceInfo) {
            serviceInfo.responseTime = responseTime;
            serviceInfo.status = 'available';
            serviceInfo.lastCheck = Date.now();
        }
        
        this.updatePerformanceMetricsUI();
        this.updateServiceStatusUI();
        
        if (this.callbacks.onMetricsUpdate) {
            this.callbacks.onMetricsUpdate(this.performanceMetrics);
        }
    }

    /**
     * 记录服务错误
     * @param {string} service - 服务名称
     * @param {string} error - 错误信息
     */
    recordServiceError(service, error) {
        console.error(`[StatusMonitor] Service error: ${service} - ${error}`);
        
        const serviceInfo = this.serviceStatus.get(service);
        if (serviceInfo) {
            serviceInfo.status = 'error';
            serviceInfo.errorCount++;
            serviceInfo.lastCheck = Date.now();
        }
        
        this.updateServiceStatusUI();
    }

    /**
     * 更新性能指标UI
     */
    updatePerformanceMetricsUI() {
        if (!this.performanceContainer) return;
        
        const totalTimeElement = document.getElementById('total-time');
        const providerRatioElement = document.getElementById('provider-ratio');
        
        if (totalTimeElement) {
            const avgTime = this.performanceMetrics.requestCount > 0 
                ? Math.round(this.performanceMetrics.totalResponseTime / this.performanceMetrics.requestCount)
                : 0;
            totalTimeElement.textContent = `${avgTime}ms`;
        }
        
        if (providerRatioElement) {
            const local = this.performanceMetrics.localRequests;
            const remote = this.performanceMetrics.remoteRequests;
            providerRatioElement.textContent = `${local}/${remote}`;
        }
    }

    /**
     * 更新AOG连接状态
     * @param {string} message - 状态消息
     * @param {string} type - 状态类型
     */
    updateAOGStatus(message, type) {
        if (!this.aogStatus) return;
        
        const statusDot = this.aogStatus.querySelector('.status-dot');
        const statusText = this.aogStatus.querySelector('.status-text');
        
        if (statusText) {
            statusText.textContent = message;
        }
        
        if (statusDot) {
            statusDot.className = 'status-dot';
            
            switch (type) {
                case 'connected':
                    statusDot.classList.add('connected');
                    break;
                case 'error':
                case 'disconnected':
                    statusDot.classList.add('error');
                    break;
                default:
                    statusDot.classList.add('connecting');
            }
        }
    }

    /**
     * 切换面板显示状态
     */
    togglePanel() {
        this.isCollapsed = !this.isCollapsed;
        
        if (this.statusPanel) {
            if (this.isCollapsed) {
                this.statusPanel.classList.add('collapsed');
            } else {
                this.statusPanel.classList.remove('collapsed');
            }
        }
        
        console.log(`[StatusMonitor] Panel ${this.isCollapsed ? 'collapsed' : 'expanded'}`);
    }

    /**
     * 获取服务统计信息
     * @returns {Object} 统计信息
     */
    getServiceStats() {
        const stats = {
            services: {},
            performance: { ...this.performanceMetrics }
        };
        
        this.serviceStatus.forEach((info, service) => {
            stats.services[service] = {
                status: info.status,
                responseTime: info.responseTime,
                errorCount: info.errorCount,
                lastCheck: info.lastCheck
            };
        });
        
        return stats;
    }

    /**
     * 重置性能指标
     */
    resetMetrics() {
        console.log('[StatusMonitor] Resetting performance metrics...');
        
        this.performanceMetrics = {
            totalResponseTime: 0,
            localRequests: 0,
            remoteRequests: 0,
            requestCount: 0
        };
        
        this.updatePerformanceMetricsUI();
    }

    /**
     * 获取可用模型列表
     */
    async getAvailableModels() {
        try {
            const response = await fetch('/api/models');
            
            if (response.ok) {
                const models = await response.json();
                console.log('[StatusMonitor] Available models:', models);
                return models;
            } else {
                throw new Error(`HTTP ${response.status}`);
            }
        } catch (error) {
            console.error('[StatusMonitor] Failed to get models:', error);
            return null;
        }
    }

    /**
     * 清理资源
     */
    cleanup() {
        console.log('[StatusMonitor] Cleaning up...');
        
        this.stopMonitoring();
        this.serviceStatus.clear();
        this.resetMetrics();
    }
}

// 导出类
window.StatusMonitor = StatusMonitor;

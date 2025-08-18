/**
 * AOG智能内容创作助手 - 步骤管理器
 * Copyright 2024-2025 Intel Corporation
 */

class StepManager {
    constructor() {
        this.currentStep = 1;
        this.maxStep = 7;
        this.stepPages = {};
        this.stepIndicator = null;
        
        this.initializeElements();
        this.bindNavigationEvents();
    }

    /**
     * 初始化DOM元素
     */
    initializeElements() {
        this.stepIndicator = document.querySelector('.step-indicator');
        
        // 获取所有步骤页面
        for (let i = 1; i <= this.maxStep; i++) {
            this.stepPages[i] = document.getElementById(`step-${i}`);
        }
        this.stepPages.complete = document.getElementById('step-complete');
        
        // 确保只有第一步显示
        this.showStep(1);
    }

    /**
     * 绑定导航事件
     */
    bindNavigationEvents() {
        // 步骤1: 语音输入
        this.bindButton('confirm-speech-btn', () => this.handleStepComplete(1));
        this.bindButton('retry-speech-btn', () => this.handleStepRetry(1));
        
        // 步骤2: 文案生成
        this.bindButton('back-to-speech-btn', () => this.goToStep(1));
        this.bindButton('confirm-content-btn', () => this.handleStepComplete(2));
        this.bindButton('regenerate-content-btn', () => this.handleStepRetry(2));
        
        // 步骤3: 摄像头拍摄
        this.bindButton('back-to-content-btn', () => this.goToStep(2));
        this.bindButton('skip-camera-btn', () => this.handleStepSkip(3));
        this.bindButton('confirm-camera-btn', () => this.handleStepComplete(3));
        
        // 步骤4: 本地图片生成
        this.bindButton('back-to-camera-btn', () => this.goToStep(3));
        this.bindButton('confirm-local-btn', () => this.handleStepComplete(4));
        this.bindButton('regenerate-local-btn', () => this.handleStepRetry(4));
        
        // 步骤5: 语音优化
        this.bindButton('back-to-local-btn', () => this.goToStep(4));
        this.bindButton('confirm-optimization-btn', () => this.handleStepComplete(5));
        this.bindButton('retry-optimization-btn', () => this.handleStepRetry(5));
        
        // 步骤6: 云端生成
        this.bindButton('back-to-optimization-btn', () => this.goToStep(5));
        this.bindButton('confirm-cloud-btn', () => this.handleStepComplete(6));
        this.bindButton('regenerate-cloud-btn', () => this.handleStepRetry(6));
        
        // 步骤7: 语音播报
        this.bindButton('back-to-cloud-btn', () => this.goToStep(6));
        this.bindButton('complete-workflow-btn', () => this.handleWorkflowComplete());
        this.bindButton('regenerate-audio-btn', () => this.handleStepRetry(7));
        
        // 完成页面
        this.bindButton('start-new-btn', () => this.startNewWorkflow());
    }

    /**
     * 绑定按钮事件
     */
    bindButton(id, handler) {
        const button = document.getElementById(id);
        if (button) {
            button.addEventListener('click', handler);
            console.log(`[StepManager] Bound event for #${id}`);
        } else {
            console.warn(`[StepManager] Button #${id} not found`);
        }
    }

    /**
     * 跳转到指定步骤
     */
    goToStep(step) {
        if (step < 1 || step > this.maxStep) return;
        
        this.currentStep = step;
        this.showStep(step);
        this.updateStepIndicator();
        
        console.log(`[StepManager] Navigated to step ${step}`);
        
        // 触发步骤进入事件
        this.triggerStepEvent('enter', step);
    }

    /**
     * 显示指定步骤
     */
    showStep(step) {
        // 隐藏所有步骤
        Object.values(this.stepPages).forEach(page => {
            if (page) page.classList.remove('active');
        });
        
        // 显示目标步骤
        const targetPage = step === 'complete' ? this.stepPages.complete : this.stepPages[step];
        if (targetPage) {
            targetPage.classList.add('active');
        }
    }

    /**
     * 更新步骤指示器
     */
    updateStepIndicator() {
        if (!this.stepIndicator) return;
        
        const steps = this.stepIndicator.querySelectorAll('.step');
        
        steps.forEach((step, index) => {
            const stepNumber = index + 1;
            step.classList.remove('active', 'completed');
            
            if (stepNumber < this.currentStep) {
                step.classList.add('completed');
            } else if (stepNumber === this.currentStep) {
                step.classList.add('active');
            }
        });
    }

    /**
     * 处理步骤完成
     */
    handleStepComplete(step) {
        console.log(`[StepManager] Step ${step} completed`);
        
        // 标记当前步骤为完成
        this.markStepCompleted(step);
        
        // 触发步骤完成事件
        this.triggerStepEvent('complete', step);
        
        // 自动进入下一步
        if (step < this.maxStep) {
            setTimeout(() => {
                this.goToStep(step + 1);
            }, 500);
        }
    }

    /**
     * 处理步骤重试
     */
    handleStepRetry(step) {
        console.log(`[StepManager] Step ${step} retry`);
        
        // 触发步骤重试事件
        this.triggerStepEvent('retry', step);
    }

    /**
     * 处理步骤跳过
     */
    handleStepSkip(step) {
        console.log(`[StepManager] Step ${step} skipped`);
        
        // 触发步骤跳过事件
        this.triggerStepEvent('skip', step);
        
        // 进入下一步
        this.handleStepComplete(step);
    }

    /**
     * 处理工作流程完成
     */
    handleWorkflowComplete() {
        console.log('[StepManager] Workflow completed');
        
        // 标记最后一步完成
        this.markStepCompleted(this.maxStep);
        
        // 显示完成页面
        this.showStep('complete');
        
        // 触发工作流程完成事件
        this.triggerStepEvent('workflow-complete', 'complete');
    }

    /**
     * 开始新的工作流程
     */
    startNewWorkflow() {
        console.log('[StepManager] Starting new workflow');
        
        // 重置到第一步
        this.currentStep = 1;
        this.goToStep(1);
        
        // 触发新工作流程事件
        this.triggerStepEvent('new-workflow', 1);
    }

    /**
     * 标记步骤为完成
     */
    markStepCompleted(step) {
        if (!this.stepIndicator) return;
        
        const stepElement = this.stepIndicator.querySelector(`[data-step="${step}"]`);
        if (stepElement) {
            stepElement.classList.remove('active');
            stepElement.classList.add('completed');
        }
    }

    /**
     * 触发步骤事件
     */
    triggerStepEvent(eventType, step) {
        const event = new CustomEvent('step-event', {
            detail: {
                type: eventType,
                step: step,
                currentStep: this.currentStep
            }
        });
        
        document.dispatchEvent(event);
    }

    /**
     * 获取当前步骤
     */
    getCurrentStep() {
        return this.currentStep;
    }

    /**
     * 检查步骤是否完成
     */
    isStepCompleted(step) {
        if (!this.stepIndicator) return false;
        
        const stepElement = this.stepIndicator.querySelector(`[data-step="${step}"]`);
        return stepElement ? stepElement.classList.contains('completed') : false;
    }

    /**
     * 启用/禁用步骤按钮
     */
    setStepButtonState(buttonId, enabled) {
        const button = document.getElementById(buttonId);
        if (button) {
            button.disabled = !enabled;
        }
    }

    /**
     * 显示/隐藏步骤按钮
     */
    setStepButtonVisibility(buttonId, visible) {
        const button = document.getElementById(buttonId);
        if (button) {
            button.style.display = visible ? 'inline-flex' : 'none';
        }
    }
}

// 导出类
window.StepManager = StepManager;

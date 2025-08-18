package manager

import (
	"context"
	"sync"
	"time"

	"github.com/intel/aog/config"
	"github.com/intel/aog/internal/logger"
	"github.com/intel/aog/internal/provider"
	"github.com/intel/aog/internal/types"
)

// isEmbedService determines if it's an embed service
func isEmbedService(serviceType string) bool {
	return serviceType == types.ServiceEmbed
}

// isLocalNonEmbedService determines if it's a local non-embed service
func isLocalNonEmbedService(location, serviceType string) bool {
	return location == types.ServiceSourceLocal && !isEmbedService(serviceType)
}

// NeedsQueuing determines if request needs to enter queuing mechanism (exported function)
func NeedsQueuing(location, serviceType string) bool {
	return isLocalNonEmbedService(location, serviceType)
}

// Manager model manager, implements ModelStateManager interface
type Manager struct {
	queue   *Queue   // Request queue
	loader  *Loader  // Model loader
	cleaner *Cleaner // Auto cleaner

	currentModel string          // Currently loaded model
	modelStates  map[string]bool // Model states: true=in use, false=idle
	mutex        sync.RWMutex    // Protect state
}

var (
	instance *Manager
	once     sync.Once
)

// GetModelManager 获取全局模型管理器实例
func GetModelManager() *Manager {
	once.Do(func() {
		instance = NewManager()
	})
	return instance
}

// NewManager 创建新的模型管理器
func NewManager() *Manager {
	manager := &Manager{
		modelStates: make(map[string]bool),
	}

	// 先创建loader
	manager.loader = NewLoader(manager)

	// 创建queue时注入接口依赖
	manager.queue = NewQueue(manager, manager.loader)

	// 创建cleaner时注入接口依赖
	manager.cleaner = NewCleaner(manager.queue, manager.loader)

	return manager
}

// GetCurrentModel 获取当前加载的模型
func (m *Manager) GetCurrentModel() string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.currentModel
}

// SetCurrentModel 设置当前模型
func (m *Manager) SetCurrentModel(modelName string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.currentModel = modelName
}

// MarkModelInUse 标记模型为使用中
func (m *Manager) MarkModelInUse(modelName string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.modelStates == nil {
		m.modelStates = make(map[string]bool)
	}

	m.modelStates[modelName] = true
	logger.LogicLogger.Debug("[Manager] Model marked as in use", "model", modelName)
	return nil
}

// MarkModelIdle 标记模型为空闲
func (m *Manager) MarkModelIdle(modelName string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.modelStates == nil {
		m.modelStates = make(map[string]bool)
	}

	m.modelStates[modelName] = false
	logger.LogicLogger.Debug("[Manager] Model marked as idle", "model", modelName)
	return nil
}

// GetModelMemoryManager 获取全局模型管理器实例（保持向后兼容）
func GetModelMemoryManager() *Manager {
	return GetModelManager()
}

// Start 启动模型管理器
func (m *Manager) Start(cleanupInterval time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 如果传入的清理间隔为0，使用配置中的默认值
	if cleanupInterval == 0 {
		if config.GlobalEnvironment != nil && config.GlobalEnvironment.ModelCleanupInterval > 0 {
			cleanupInterval = config.GlobalEnvironment.ModelCleanupInterval
		} else {
			cleanupInterval = 1 * time.Minute // 默认1分钟
		}
	}

	// 初始化时清理已运行的模型（异步执行，不阻塞启动）
	logger.LogicLogger.Info("[Manager] Starting cleanup of running models from providers...")
	go m.loader.InitializeRunningModels()

	// 启动队列
	m.queue.Start()

	// 启动清理器
	m.cleaner.Start(cleanupInterval)

	logger.LogicLogger.Info("[Manager] Started",
		"cleanup_interval", cleanupInterval)
}

// Stop 停止模型管理器
func (m *Manager) Stop() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 停止队列
	m.queue.Stop()

	// 停止清理器
	m.cleaner.Stop()

	logger.LogicLogger.Info("[Manager] Stopped")
}

// SetIdleTimeout 设置空闲超时时间
func (m *Manager) SetIdleTimeout(timeout time.Duration) {
	m.cleaner.SetIdleTimeout(timeout)
}

// GetModelState 获取模型状态信息
func (m *Manager) GetModelState(modelName string) (*ModelState, bool) {
	return m.loader.GetModelState(modelName)
}

// GetAllModelStates 获取所有模型状态信息
func (m *Manager) GetAllModelStates() map[string]*ModelState {
	return m.loader.GetAllModelStates()
}

// ForceUnloadModel 强制卸载指定模型
func (m *Manager) ForceUnloadModel(modelName string) error {
	return m.loader.ForceUnloadModel(modelName)
}

// EnqueueLocalModelRequest 将本地模型请求加入队列，返回准备完成通道和错误通道
func (m *Manager) EnqueueLocalModelRequest(ctx context.Context, modelName string, providerInstance provider.ModelServiceProvider, providerName, providerType string, taskID uint64) (chan struct{}, chan error, error) {
	// 创建排队请求
	request := &QueuedRequest{
		TaskID:       taskID,
		ModelName:    modelName,
		Provider:     providerInstance,
		ProviderName: providerName,
		ProviderType: providerType,
		Context:      ctx,
		StartTime:    time.Now(),
		ReadyChan:    make(chan struct{}),
		CompleteChan: make(chan struct{}),
		ErrorChan:    make(chan error, 1),
	}

	// 加入队列（非阻塞）
	if err := m.queue.EnqueueRequest(request); err != nil {
		return nil, nil, err
	}

	logger.LogicLogger.Debug("[Manager] Local model request enqueued",
		"task_id", taskID,
		"model", modelName)

	return request.ReadyChan, request.ErrorChan, nil
}

// CompleteLocalModelRequest 完成本地模型请求处理
func (m *Manager) CompleteLocalModelRequest(taskID uint64) {
	logger.LogicLogger.Debug("[Manager] Completing local model request", "task_id", taskID)
	m.queue.CompleteRequest(taskID)
}

// GetStats 获取管理器统计信息
func (m *Manager) GetStats() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	stats := map[string]interface{}{
		"queue":   m.queue.GetStats(),
		"loader":  m.loader.GetStats(),
		"cleaner": m.cleaner.GetStats(),
	}

	return stats
}

package manager

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/intel/aog/internal/logger"
	"github.com/intel/aog/internal/provider"
)

// QueuedRequest queued request
type QueuedRequest struct {
	TaskID       uint64                        // Task ID
	ModelName    string                        // Model name
	Provider     provider.ModelServiceProvider // Provider instance
	ProviderName string                        // Provider name
	ProviderType string                        // Provider type
	Context      context.Context               // Context
	StartTime    time.Time                     // Start time
	ReadyChan    chan struct{}                 // Model ready notification channel
	CompleteChan chan struct{}                 // Task execution completion notification channel
	ErrorChan    chan error                    // Error notification channel
}

// Queue model request queue
type Queue struct {
	queue           chan *QueuedRequest // Request queue
	processingQueue chan *QueuedRequest // Capacity 1, ensures serial processing
	currentRequest  *QueuedRequest      // Currently processing request
	processing      bool                // Whether processing request
	queueTimeout    time.Duration       // Queue timeout duration
	started         bool                // Whether started
	stopChan        chan struct{}       // Stop signal
	mutex           sync.Mutex          // Mutex lock

	// Interface dependencies, avoid circular dependencies
	stateManager ModelStateManager
	loader       ModelLoader
}

// NewQueue creates a new queue
func NewQueue(stateManager ModelStateManager, loader ModelLoader) *Queue {
	return &Queue{
		queue:           make(chan *QueuedRequest, 100),
		processingQueue: make(chan *QueuedRequest, 1), // Capacity 1, ensures serial processing
		queueTimeout:    30 * time.Second,
		stopChan:        make(chan struct{}),
		stateManager:    stateManager,
		loader:          loader,
	}
}

// Start starts queue processing
func (q *Queue) Start() {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	if q.started {
		logger.LogicLogger.Warn("[Queue] Already started")
		return
	}

	q.started = true
	go q.processLoop()

	logger.LogicLogger.Info("[Queue] Started",
		"queue_timeout", q.queueTimeout)
}

// Stop stops queue processing
func (q *Queue) Stop() {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	if !q.started {
		return
	}

	close(q.stopChan)
	q.started = false

	logger.LogicLogger.Info("[Queue] Stopped")
}

// EnqueueRequest 将请求加入队列（非阻塞）
func (q *Queue) EnqueueRequest(request *QueuedRequest) error {
	if !q.started {
		return fmt.Errorf("queue not started")
	}

	logger.LogicLogger.Debug("[Queue] Enqueueing request",
		"task_id", request.TaskID,
		"model", request.ModelName,
		"queue_length", len(q.queue))

	// 非阻塞加入队列
	select {
	case q.queue <- request:
		logger.LogicLogger.Debug("[Queue] Request enqueued successfully",
			"task_id", request.TaskID,
			"model", request.ModelName)
		return nil
	case <-time.After(q.queueTimeout):
		return fmt.Errorf("failed to enqueue request: queue full, timeout after %v", q.queueTimeout)
	case <-request.Context.Done():
		return request.Context.Err()
	}
}

// CompleteRequest 完成请求处理
func (q *Queue) CompleteRequest(taskID uint64) {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	if q.currentRequest != nil && q.currentRequest.TaskID == taskID {
		logger.LogicLogger.Debug("[Queue] Completing request",
			"task_id", taskID,
			"model", q.currentRequest.ModelName,
			"total_time", time.Since(q.currentRequest.StartTime))

		// 关闭CompleteChan，通知队列任务真正完成
		close(q.currentRequest.CompleteChan)
		logger.LogicLogger.Debug("[Queue] CompleteChan closed, task execution completed", "task_id", taskID)

		// 标记模型为空闲（如果没有其他请求在等待）
		if len(q.processingQueue) == 0 {
			currentModel := q.stateManager.GetCurrentModel()
			if currentModel != "" {
				err := q.stateManager.MarkModelIdle(currentModel)
				if err != nil {
					logger.LogicLogger.Warn("[Queue] Failed to mark model idle",
						"model", currentModel, "error", err)
				}
			}
		}

		q.processing = false
		q.currentRequest = nil

		logger.LogicLogger.Debug("[Queue] Request completed and queue ready for next",
			"task_id", taskID)
	}
}

// processLoop 队列处理循环
func (q *Queue) processLoop() {
	logger.LogicLogger.Debug("[Queue] Process loop started")

	for {
		select {
		case request := <-q.queue:
			// 将请求加入串行处理队列（容量为1，天然串行）
			logger.LogicLogger.Debug("[Queue] Request received, attempting to process",
				"task_id", request.TaskID, "model", request.ModelName)

			select {
			case q.processingQueue <- request:
				logger.LogicLogger.Debug("[Queue] Request moved to processing queue",
					"task_id", request.TaskID, "model", request.ModelName)
			case <-q.stopChan:
				logger.LogicLogger.Debug("[Queue] Process loop stopped while queuing request")
				return
			}

		case request := <-q.processingQueue:
			// 串行处理请求
			q.processRequestSerial(request)

			// 等待任务真正完成后再处理下一个请求
			select {
			case <-request.CompleteChan:
				logger.LogicLogger.Debug("[Queue] Task execution completed, ready for next request",
					"task_id", request.TaskID)
			case <-q.stopChan:
				logger.LogicLogger.Debug("[Queue] Process loop stopped while waiting for task completion")
				return
			}

		case <-q.stopChan:
			logger.LogicLogger.Debug("[Queue] Process loop stopped")
			return
		}
	}
}

// processRequestSerial 串行处理单个请求
func (q *Queue) processRequestSerial(request *QueuedRequest) {
	q.mutex.Lock()
	q.processing = true
	q.currentRequest = request
	q.mutex.Unlock()

	logger.LogicLogger.Debug("[Queue] Processing model request",
		"task_id", request.TaskID,
		"model", request.ModelName,
		"queue_wait_time", time.Since(request.StartTime))

	// 检查是否需要切换模型
	currentModel := q.stateManager.GetCurrentModel()
	if currentModel != request.ModelName {
		logger.LogicLogger.Info("[Queue] Model switch required",
			"from", currentModel, "to", request.ModelName,
			"task_id", request.TaskID)

		logger.LogicLogger.Debug("[Queue] Calling SwitchModel",
			"model", request.ModelName, "task_id", request.TaskID)

		err := q.loader.SwitchModel(request.ModelName, request.Provider)
		if err != nil {
			logger.LogicLogger.Error("[Queue] SwitchModel failed",
				"model", request.ModelName, "error", err, "task_id", request.TaskID)

			// 清理状态
			q.mutex.Lock()
			q.processing = false
			q.currentRequest = nil
			q.mutex.Unlock()

			// 发送错误到ErrorChan，然后关闭ReadyChan
			select {
			case request.ErrorChan <- err:
				logger.LogicLogger.Debug("[Queue] Error sent to ErrorChan", "task_id", request.TaskID)
			default:
				logger.LogicLogger.Warn("[Queue] Failed to send error to ErrorChan", "task_id", request.TaskID)
			}
			close(request.ReadyChan)
			logger.LogicLogger.Debug("[Queue] ReadyChan closed due to error", "task_id", request.TaskID)
			return
		}

		logger.LogicLogger.Info("[Queue] Model switch completed successfully",
			"from", currentModel, "to", request.ModelName, "task_id", request.TaskID)
	} else {
		logger.LogicLogger.Debug("[Queue] No model switch needed, model already loaded",
			"model", request.ModelName, "task_id", request.TaskID)
	}

	// 标记模型使用中
	err := q.stateManager.MarkModelInUse(request.ModelName)
	if err != nil {
		logger.LogicLogger.Warn("[Queue] Failed to mark model in use",
			"model", request.ModelName, "error", err, "task_id", request.TaskID)
	}

	logger.LogicLogger.Info("[Queue] Model request ready for processing",
		"task_id", request.TaskID,
		"model", request.ModelName,
		"provider", request.ProviderName)

	// 通知调度器模型已准备完成，可以开始执行任务
	close(request.ReadyChan)
	logger.LogicLogger.Debug("[Queue] ReadyChan closed, model ready for task execution", "task_id", request.TaskID)
}

// GetStats 获取队列统计信息
func (q *Queue) GetStats() map[string]interface{} {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	stats := map[string]interface{}{
		"queue_length":      len(q.queue),
		"processing_length": len(q.processingQueue),
		"queue_timeout":     q.queueTimeout.String(),
		"processing":        q.processing,
		"started":           q.started,
	}

	if q.currentRequest != nil {
		stats["current_request"] = map[string]interface{}{
			"task_id":    q.currentRequest.TaskID,
			"model_name": q.currentRequest.ModelName,
			"start_time": q.currentRequest.StartTime,
		}
	}

	return stats
}

// IsProcessing 检查是否正在处理请求
func (q *Queue) IsProcessing() bool {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	return q.processing
}

// HasPendingRequests 检查是否有待处理的请求
func (q *Queue) HasPendingRequests() bool {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	return len(q.processingQueue) > 0
}

// HasActiveRequests 检查是否有活跃的请求
func (q *Queue) HasActiveRequests() bool {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	return q.processing || len(q.processingQueue) > 0
}

// GetCurrentRequest 获取当前处理的请求
func (q *Queue) GetCurrentRequest() *QueuedRequest {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	return q.currentRequest
}

// GetQueueLength 获取当前队列长度
func (q *Queue) GetQueueLength() int {
	return len(q.queue)
}

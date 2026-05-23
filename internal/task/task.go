package task

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/lingyuins/octopus/internal/utils/log"
)

type taskEntry struct {
	name       string
	interval   time.Duration
	fn         func()
	runOnStart bool
	ticker     *time.Ticker
	stopCh     chan struct{}
	updateCh   chan time.Duration
	running    atomic.Bool
}

var (
	tasks          = make(map[string]*taskEntry)
	tasksMu        sync.RWMutex
	taskShutdownCh = make(chan struct{})
)

// Shutdown signals all registered tasks to stop and waits for them to finish.
func Shutdown() {
	close(taskShutdownCh)
	tasksMu.RLock()
	defer tasksMu.RUnlock()
	for _, entry := range tasks {
		select {
		case <-entry.stopCh:
		default:
			close(entry.stopCh)
		}
	}
	log.Infof("all background tasks have been stopped")
}

// Register 注册一个定时任务
// runOnStart: 是否在启动时立即执行一次
func Register(name string, interval time.Duration, runOnStart bool, fn func()) {
	if interval <= 0 {
		log.Debugf("task %s not registered: interval is 0", name)
		return
	}

	tasksMu.Lock()
	defer tasksMu.Unlock()

	if _, exists := tasks[name]; exists {
		log.Warnf("task %s already registered, skipping", name)
		return
	}

	tasks[name] = &taskEntry{
		name:       name,
		interval:   interval,
		fn:         fn,
		runOnStart: runOnStart,
		stopCh:     make(chan struct{}),
		updateCh:   make(chan time.Duration),
	}
	log.Debugf("task %s registered with interval %v, runOnStart: %v", name, interval, runOnStart)
}

// Update 更新任务的执行间隔
// 当 interval 为 0 时，删除任务
func Update(name string, interval time.Duration) {
	tasksMu.Lock()
	entry, exists := tasks[name]
	if !exists {
		tasksMu.Unlock()
		log.Warnf("task %s not found", name)
		return
	}

	if interval <= 0 {
		delete(tasks, name)
		tasksMu.Unlock()
		close(entry.stopCh)
		log.Infof("task %s removed: interval is 0", name)
		return
	}
	tasksMu.Unlock()

	select {
	case entry.updateCh <- interval:
		log.Infof("task %s interval updated to %v", name, interval)
	default:
		log.Warnf("task %s update channel full, skipping", name)
	}
}

// RUN 启动所有注册的任务
func RUN() {
	tasksMu.RLock()
	for _, entry := range tasks {
		go runTask(entry)
	}
	tasksMu.RUnlock()

	// 阻塞主协程直到收到关闭信号
	<-taskShutdownCh
}

func runTask(entry *taskEntry) {
	runOnce := func() {
		if !entry.running.CompareAndSwap(false, true) {
			log.Warnf("task %s is still running, skipping overlapping run", entry.name)
			return
		}
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Errorf("task %s panic recovered: %v", entry.name, r)
				}
			}()
			defer entry.running.Store(false)
			entry.fn()
		}()
	}

	if entry.runOnStart {
		runOnce()
	}

	entry.ticker = time.NewTicker(entry.interval)
	defer entry.ticker.Stop()

	for {
		select {
		case <-entry.ticker.C:
			runOnce()
		case newInterval := <-entry.updateCh:
			entry.ticker.Stop()
			entry.interval = newInterval
			entry.ticker = time.NewTicker(newInterval)
		case <-entry.stopCh:
			return
		}
	}
}

package execution

import "sync"

// createRequestRegistry 只封住权威首发事实提交前的进程内并发窗口。
type createRequestRegistry struct {
	// mutex 保护 active 的检查与变更。
	mutex sync.Mutex
	// active 保存当前正在准备但尚未确定提交结果的稳定资源标识。
	active map[string]struct{}
}

// newCreateRequestRegistry 创建空的首次创建请求注册表。
func newCreateRequestRegistry() *createRequestRegistry {
	return &createRequestRegistry{active: make(map[string]struct{})}
}

// reserve 尝试独占一个稳定资源标识。
func (r *createRequestRegistry) reserve(resourceID string) bool {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if _, exists := r.active[resourceID]; exists {
		return false
	}
	r.active[resourceID] = struct{}{}
	return true
}

// release 释放尚未进入或已经进入权威事实的进程内准备占位。
func (r *createRequestRegistry) release(resourceID string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	delete(r.active, resourceID)
}

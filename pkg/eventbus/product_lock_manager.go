package eventbus

import "sync"

// ProductLockManager provides per-product locking for ordered event processing.
// Events for the same product are serialized; different products process in parallel.
type ProductLockManager struct {
	locks sync.Map // map[productID]*sync.Mutex
}

// NewProductLockManager creates a new ProductLockManager instance.
func NewProductLockManager() *ProductLockManager {
	return &ProductLockManager{}
}

// Lock acquires a lock for the given product ID.
// If another goroutine has the lock, this will block until it's released.
func (m *ProductLockManager) Lock(productID string) {
	lock, _ := m.locks.LoadOrStore(productID, &sync.Mutex{})
	lock.(*sync.Mutex).Lock()
}

// Unlock releases the lock for the given product ID.
func (m *ProductLockManager) Unlock(productID string) {
	if lock, ok := m.locks.Load(productID); ok {
		lock.(*sync.Mutex).Unlock()
	}
}

// WithLock executes the given function while holding the lock for the product ID.
// This is a convenience method that ensures the lock is always released.
func (m *ProductLockManager) WithLock(productID string, fn func()) {
	m.Lock(productID)
	defer m.Unlock(productID)
	fn()
}

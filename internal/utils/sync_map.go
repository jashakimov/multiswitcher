package utils

import "sync"

func NewSyncMap[K comparable, V comparable]() *SyncMap[K, V] {
	return &SyncMap[K, V]{
		items: make(map[K]V),
		lock:  sync.RWMutex{},
	}
}

type SyncMap[K comparable, V comparable] struct {
	items map[K]V
	lock  sync.RWMutex
}

func (m *SyncMap[K, V]) Set(key K, value V) {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.items[key] = value
}

func (m *SyncMap[K, V]) Get(key K) (V, bool) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	val, ok := m.items[key]

	return val, ok
}

func (m *SyncMap[K, V]) Values() map[K]V {
	m.lock.RLock()
	defer m.lock.RUnlock()

	return m.items
}

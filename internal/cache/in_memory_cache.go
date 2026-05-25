package cache

import "sync"

type InMemoryCache[K comparable, V any] struct {
	mu   sync.RWMutex
	data map[K]V
}

func NewInMemoryCache[K comparable, V any]() Cache[K, V] {
	return &InMemoryCache[K, V]{
		data: make(map[K]V),
	}
}

func (c *InMemoryCache[K, V]) Get(key K) (V, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	val, ok := c.data[key]
	return val, ok
}

func (c *InMemoryCache[K, V]) Set(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data[key] = value
}

func (c *InMemoryCache[K, V]) Delete(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.data, key)
}

func (c *InMemoryCache[K, V]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data = make(map[K]V)
}
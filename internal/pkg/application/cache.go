package application

import (
	"sync"
	"time"
)

type CacheItem struct {
	Value      any
	ExpiryTime time.Time
}

type Cache struct {
	items map[string]CacheItem
	mutex sync.RWMutex
}

func NewCache() *Cache {
	return &Cache{
		items: make(map[string]CacheItem),
	}
}

func (c *Cache) Set(key string, value any, duration time.Duration) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.items[key] = CacheItem{
		Value:      value,
		ExpiryTime: time.Now().Add(duration),
	}
}

func (c *Cache) Get(key string) (any, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	item, exists := c.items[key]
	if !exists || item.ExpiryTime.Before(time.Now()) {
		return nil, false
	}

	return item.Value, true
}

func (c *Cache) Cleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for {
			<-ticker.C
			now := time.Now()
			c.mutex.Lock()
			for key, item := range c.items {
				if item.ExpiryTime.Before(now) {
					delete(c.items, key)
				}
			}
			c.mutex.Unlock()
		}
	}()
}

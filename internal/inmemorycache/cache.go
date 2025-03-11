package inmemorycache

import (
	"encoding/json"
	"sync"
	"time"
)

type WeatherCacheData struct {
	Temperature float64 `json:"temperature"`
	Warning     string  `json:"warning,omitempty"`
}

type cacheEntry struct {
	data       []byte
	expiration time.Time
}

type Cache interface {
	Get(location string) (*WeatherCacheData, bool, error)
	Set(location string, data *WeatherCacheData, ttl time.Duration) error
}

type InMemoryCache struct {
	cache           map[string]cacheEntry
	mutex           sync.Mutex
	cleanupInterval time.Duration
}

func NewInMemoryCacheProvider(cleanupInterval time.Duration) *InMemoryCache {
	provider := &InMemoryCache{
		cache:           make(map[string]cacheEntry),
		cleanupInterval: cleanupInterval,
	}

	go provider.startCleanup()

	return provider
}

func (m *InMemoryCache) Get(location string) (*WeatherCacheData, bool, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	entry, exists := m.cache[location]
	if !exists {
		return nil, false, nil
	}

	if time.Now().After(entry.expiration) {
		delete(m.cache, location)
		return nil, false, nil
	}

	var data WeatherCacheData
	if err := json.Unmarshal(entry.data, &data); err != nil {
		return nil, false, err
	}

	return &data, true, nil
}

func (m *InMemoryCache) Set(location string, data *WeatherCacheData, ttl time.Duration) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.cache[location] = cacheEntry{
		data:       jsonData,
		expiration: time.Now().Add(ttl),
	}

	return nil
}

func (m *InMemoryCache) startCleanup() {
	ticker := time.NewTicker(m.cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		m.mutex.Lock()
		now := time.Now()
		for k, v := range m.cache {
			if now.After(v.expiration) {
				delete(m.cache, k)
			}
		}
		m.mutex.Unlock()
	}
}

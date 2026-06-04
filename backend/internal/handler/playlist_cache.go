package handler

import (
	"strings"
	"sync"
	"time"
)

const playlistCacheTTL = 60 * time.Second

type playlistBodyCache struct {
	ttl time.Duration
	mu  sync.RWMutex
	m   map[string]playlistCacheEntry
}

type playlistCacheEntry struct {
	body []byte
	at   time.Time
}

func newPlaylistBodyCache() *playlistBodyCache {
	return &playlistBodyCache{
		ttl: playlistCacheTTL,
		m:   make(map[string]playlistCacheEntry),
	}
}

func playlistCacheKey(videoID, path string) string {
	return videoID + "|" + path
}

func (c *playlistBodyCache) Get(videoID, path string) ([]byte, bool) {
	key := playlistCacheKey(videoID, path)
	c.mu.RLock()
	ent, ok := c.m[key]
	c.mu.RUnlock()
	if !ok || time.Since(ent.at) >= c.ttl {
		return nil, false
	}
	return ent.body, true
}

func (c *playlistBodyCache) Set(videoID, path string, body []byte) {
	key := playlistCacheKey(videoID, path)
	dup := make([]byte, len(body))
	copy(dup, body)
	c.mu.Lock()
	c.m[key] = playlistCacheEntry{body: dup, at: time.Now()}
	c.mu.Unlock()
}

func (c *playlistBodyCache) InvalidateVideo(videoID string) {
	prefix := videoID + "|"
	c.mu.Lock()
	for k := range c.m {
		if strings.HasPrefix(k, prefix) {
			delete(c.m, k)
		}
	}
	c.mu.Unlock()
}

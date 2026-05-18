package processor

import "sync"

type Deduplicator struct {
	mu       sync.RWMutex
	urlMap   map[string]bool
	slugMap  map[string]bool
}

func NewDeduplicator() *Deduplicator {
	return &Deduplicator{
		urlMap: make(map[string]bool),
		slugMap: make(map[string]bool),
	}
}

func (d *Deduplicator) LoadFromMap(existingSlugs map[string]bool) {
	d.mu.Lock()
	defer d.mu.Unlock()

	for slug := range existingSlugs {
		d.slugMap[slug] = true
	}
}

func (d *Deduplicator) IsDuplicate(url, slug string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if d.urlMap[url] {
		return true
	}

	if d.slugMap[slug] {
		return true
	}

	return false
}

func (d *Deduplicator) MarkAsProcessed(url, slug string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.urlMap[url] = true
	d.slugMap[slug] = true
}

func (d *Deduplicator) Count() int {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return len(d.urlMap)
}
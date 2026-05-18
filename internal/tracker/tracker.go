package tracker

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"time"
)

type Tracker struct {
	filePath string
	Articles map[string]Article `json:"articles"`
}

type Article struct {
	Title      string    `json:"title"`
	Link       string    `json:"link"`
	CMSID      string    `json:"cmsId"`
	CreatedAt time.Time `json:"createdAt"`
}

func NewTracker() *Tracker {
	dir, _ := os.Getwd()
	filePath := filepath.Join(dir, "article_tracker.json")
	
	tracker := &Tracker{
		filePath: filePath,
		Articles: make(map[string]Article),
	}
	
	tracker.load()
	return tracker
}

func (t *Tracker) load() {
	data, err := os.ReadFile(t.filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("Tracker: Could not load file: %v", err)
		}
		return
	}
	
	var saved struct {
		Articles map[string]Article `json:"articles"`
	}
	
	if err := json.Unmarshal(data, &saved); err != nil {
		log.Printf("Tracker: Could not parse file: %v", err)
		return
	}
	
	t.Articles = saved.Articles
	log.Printf("Tracker: Loaded %d existing articles", len(t.Articles))
}

func (t *Tracker) save() {
	data, err := json.MarshalIndent(struct {
		Articles map[string]Article `json:"articles"`
	}{
		Articles: t.Articles,
	}, "", "  ")
	
	if err != nil {
		log.Printf("Tracker: Could not save: %v", err)
		return
	}
	
	err = os.WriteFile(t.filePath, data, 0644)
	if err != nil {
		log.Printf("Tracker: Could not write file: %v", err)
	}
}

func (t *Tracker) IsDuplicate(link, title string) bool {
	// Check by link
	if _, exists := t.Articles[link]; exists {
		return true
	}
	
	// Check by similar title (rough check)
	for _, art := range t.Articles {
		if art.Title == title {
			return true
		}
	}
	
	return false
}

func (t *Tracker) Add(link, title, cmsID string) {
	t.Articles[link] = Article{
		Title:      title,
		Link:       link,
		CMSID:      cmsID,
		CreatedAt:  time.Now(),
	}
	
	// Cleanup old entries if we have too many (keep last 5000)
	if len(t.Articles) > 5000 {
		t.cleanupOldEntries()
	}
	
	t.save()
	log.Printf("Tracker: Added %s (total: %d)", title, len(t.Articles))
}

func (t *Tracker) cleanupOldEntries() {
	cutoff := time.Now().AddDate(0, 0, -30) // 30 days ago
	
	var toDelete []string
	for link, art := range t.Articles {
		if art.CreatedAt.Before(cutoff) {
			toDelete = append(toDelete, link)
		}
	}
	
	for _, link := range toDelete {
		delete(t.Articles, link)
	}
	
	if len(toDelete) > 0 {
		log.Printf("Tracker: Cleaned up %d old articles", len(toDelete))
	}
}

func (t *Tracker) Count() int {
	return len(t.Articles)
}
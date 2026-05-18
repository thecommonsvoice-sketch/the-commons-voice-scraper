package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"scraper/internal/client"
	"scraper/internal/config"
	"scraper/internal/fetcher"
	"scraper/internal/models"
	"scraper/internal/processor"
	"scraper/internal/summarizer"
	"scraper/internal/tracker"
	"scraper/internal/uploader"
)

func main() {
	log.Println("========================================")
	log.Println("  AI Reporter - Starting up...")
	log.Println("========================================")

	cfg := config.Load()

	if err := validateConfig(cfg); err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	apiClient := client.NewAPIClient(cfg)

	log.Println("Logging in as reporter...")
	if err := apiClient.Login(cfg.ReporterEmail, cfg.ReporterPass); err != nil {
		log.Fatalf("Login failed: %v", err)
	}

	log.Println("Fetching categories...")
	categories, err := apiClient.GetCategories()
	if err != nil {
		log.Fatalf("Failed to get categories: %v", err)
	}
	log.Printf("Found %d categories", len(categories))

	catMapper := processor.NewCategoryMapper(categories)

	// Initialize local tracker for deduplication
	tracker := tracker.NewTracker()
	log.Printf("Local tracker has %d articles", tracker.Count())

	rssFetcher := fetcher.NewFetcher()
	groqClient := summarizer.NewGroqClient(cfg.GroqAPIKey)
	cloudUploader := uploader.NewUploader(
		cfg.CloudinaryURL,
		cfg.CloudinaryUploadPreset,
	)
	cloudUploader.SetPixabayKey(cfg.PixabayAPIKey)
	cloudUploader.SetPexelsKey(cfg.PexelsAPIKey)

	log.Printf("Configuration loaded:")
	log.Printf("  - API Base URL: %s", cfg.APIBaseURL)
	log.Printf("  - Schedule Interval: %d minutes", cfg.ScheduleIntervalMinutes)
	log.Printf("  - Groq API: %s", boolToYesNo(cfg.GroqAPIKey != ""))
	log.Printf("  - Cloudinary: %s", boolToYesNo(cfg.CloudinaryURL != ""))
	log.Printf("  - Max Articles per run: 24", cfg.APIBaseURL)
	log.Printf("  - Pixabay: %s", boolToYesNo(cfg.PixabayAPIKey != ""))

	log.Println("========================================")
	log.Println("  Starting scraper loop...")
	log.Println("========================================")

	runScrape(rssFetcher, apiClient, catMapper, tracker, groqClient, cloudUploader)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Received shutdown signal, exiting gracefully...")
		os.Exit(0)
	}()

	ticker := time.NewTicker(cfg.ScheduleInterval())
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			log.Println("Running scheduled scrape...")
			runScrape(rssFetcher, apiClient, catMapper, tracker, groqClient, cloudUploader)
		}
	}
}

func runScrape(
	fetcher *fetcher.Fetcher,
	apiClient *client.APIClient,
	catMapper *processor.CategoryMapper,
	tracker *tracker.Tracker,
	summarizer *summarizer.GroqClient,
	imageUploader *uploader.Uploader,
) {
	startTime := time.Now()

	log.Println("Fetching RSS feeds...")
	items := fetcher.FetchAll()
	log.Printf("Found %d total items from RSS feeds", len(items))

	// Step 1: Filter out duplicates using local tracker
	var newItems []models.RSSItem
	for _, item := range items {
		if item.Title == "" || item.Link == "" {
			continue
		}
		if tracker.IsDuplicate(item.Link, item.Title) {
			log.Printf("Skipping duplicate: %s", item.Title)
			continue
		}
		newItems = append(newItems, item)
	}
	log.Printf("After dedup: %d new articles", len(newItems))

	// Step 2: AI Scores and ranks articles
	log.Println("AI analyzing articles to select the best candidates...")
	scoredItems := rankAndSelectArticles(newItems, summarizer)
	// Keep a larger pool (up to 50 scored & sorted) so every category gets a chance
	poolSize := len(scoredItems)
	if poolSize > 50 {
		poolSize = 50
	}
	scoredItems = scoredItems[:poolSize]

	log.Printf("AI scored and ranked %d articles (keeping pool of %d for category distribution)", len(scoredItems), poolSize)
	for i, item := range scoredItems {
		log.Printf("  %d. %s (score: %d)", i+1, item.Title, item.Score)
	}

	// Step 3: Categorize each scored article by content, group by category
	navbarSlugs := []string{"general", "politics", "science-and-technology", "sports-and-entertainment", "business", "world", "defence"}

	type scoredCat struct {
		item ScoredItem
		slug string
	}
	catGroups := map[string][]ScoredItem{}
	for _, si := range scoredItems {
		slug := processor.CategorizeByContent(si.Title, si.Description, si.Source)
		catGroups[slug] = append(catGroups[slug], si)
	}

	var toProcess []scoredCat
	picked := map[string]bool{} // keyed by Link

	// Round 1: pick 1 best article from each navbar category (guarantees coverage)
	for _, slug := range navbarSlugs {
		group := catGroups[slug]
		for k := 0; k < len(group); k++ {
			if !picked[group[k].Link] {
				picked[group[k].Link] = true
				toProcess = append(toProcess, scoredCat{item: group[k], slug: slug})
				break
			}
		}
	}

	// Round 2: pick 2 more from each category (total 3 per category = 21)
	for _, slug := range navbarSlugs {
		group := catGroups[slug]
		taken := 0
		for k := 0; k < len(group) && taken < 2; k++ {
			if !picked[group[k].Link] {
				picked[group[k].Link] = true
				toProcess = append(toProcess, scoredCat{item: group[k], slug: slug})
				taken++
			}
		}
	}

	// Round 3: fill up to 24 with best remaining scored articles
	for _, si := range scoredItems {
		if len(toProcess) >= 24 {
			break
		}
		if picked[si.Link] {
			continue
		}
		picked[si.Link] = true
		slug := processor.CategorizeByContent(si.Title, si.Description, si.Source)
		toProcess = append(toProcess, scoredCat{item: si, slug: slug})
	}

	log.Printf("Selected %d articles across categories", len(toProcess))
	for _, ci := range toProcess {
		log.Printf("  [%s] %s (score: %d)", ci.slug, ci.item.Title, ci.item.Score)
	}

	log.Printf("Selected %d articles across categories", len(toProcess))
	for _, ci := range toProcess {
		log.Printf("  [%s] %s (score: %d)", ci.slug, ci.item.Title, ci.item.Score)
	}

	created := 0
	skipped := 0

	for i, ci := range toProcess {
		item := ci.item
		log.Printf("[%d/%d] [%s] Processing: %s", i+1, len(toProcess), ci.slug, item.Title)

		summary, err := summarizer.GenerateSummary(item.Title, item.Description, item.Source)
		if err != nil {
			log.Printf("Failed to generate summary: %v", err)
			summary, _ = summarizer.GenerateSummary(item.Title, "", item.Source)
		}

		// Try to get image from article page first (og:image)
		coverImage := ""
		articleImage := imageUploader.FetchImageFromArticle(item.Link, item.Title)
		if articleImage != "" {
			log.Printf("Found article image for: %s", item.Title)
			coverImage = imageUploader.UploadFromURL(articleImage, item.Title)
		}
		
		// If article fetch failed, try search
		if coverImage == "" {
			log.Printf("Searching image for: %s", item.Title)
			coverImage = imageUploader.SearchAndUploadImage(item.Title)
		}

		categoryID := catMapper.GetCategoryIDFromSlug(ci.slug)
		if categoryID == "" {
			categoryID = catMapper.GetCategoryIDFromSlug("general")
		}

		keywords := processor.ExtractKeywords(item.Title)

		articleReq := &models.CreateArticleRequest{
			Title:           item.Title,
			Content:         summary,
			CategoryID:      categoryID,
			CoverImage:      coverImage,
			MetaTitle:       truncate(item.Title, 60),
			MetaDescription: truncate(item.Description, 160),
			Tags:            append(keywords, "ai-selected", strings.ToLower(item.Source)),
			Status:          "DRAFT",
		}

		createdArticle, err := apiClient.CreateArticle(articleReq)
		if err != nil {
			log.Printf("Failed to create article: %v", err)
			continue
		}

		if createdArticle != nil {
			// Add to local tracker
			tracker.Add(item.Link, item.Title, createdArticle.ID)
			created++
			log.Printf("Created article: %s (score: %d)", createdArticle.ID, item.Score)
		}

		// Rate limiting between articles
		time.Sleep(2 * time.Second)
	}

	elapsed := time.Since(startTime)
	log.Println("========================================")
	log.Printf("  Scrape completed in %.2f seconds", elapsed.Seconds())
	log.Printf("  Total scanned: %d", len(items))
	log.Printf("  AI selected: %d", len(scoredItems))
	log.Printf("  Created: %d articles", created)
	log.Printf("  Skipped: %d (duplicates/invalid)", skipped)
	log.Println("========================================")
}

type ScoredItem struct {
	models.RSSItem
	Score int
}

// rankAndSelectArticles uses AI to score and select top articles (keeps up to 50 for category distribution)
func rankAndSelectArticles(items []models.RSSItem, summarizer *summarizer.GroqClient) []ScoredItem {
	// If no Groq, use simple scoring
	if summarizer == nil {
		return simpleScore(items, 24)
	}

	var scoredItems []ScoredItem

	// Score each article (limit to first 50 to save API calls)
	maxScore := 50
	if len(items) < maxScore {
		maxScore = len(items)
	}

	for i := 0; i < maxScore; i++ {
		item := items[i]

		// Score based on keywords and topic
		score := calculateArticleScore(item)

		scoredItems = append(scoredItems, ScoredItem{
			RSSItem: item,
			Score:   score,
		})
	}

	// Sort by score (highest first)
	for i := 0; i < len(scoredItems)-1; i++ {
		for j := i + 1; j < len(scoredItems); j++ {
			if scoredItems[j].Score > scoredItems[i].Score {
				scoredItems[i], scoredItems[j] = scoredItems[j], scoredItems[i]
			}
		}
	}

	// Return top 50 for category distribution (will be trimmed after ensuring 1 per category)
	if len(scoredItems) > 50 {
		scoredItems = scoredItems[:50]
	}

	return scoredItems
}

// simpleScore for when AI is not available
func simpleScore(items []models.RSSItem, limit int) []ScoredItem {
	var scored []ScoredItem
	for _, item := range items {
		score := calculateArticleScore(item)
		scored = append(scored, ScoredItem{RSSItem: item, Score: score})
	}

	// Sort
	for i := 0; i < len(scored)-1; i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].Score > scored[i].Score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	if len(scored) > limit {
		scored = scored[:limit]
	}

	return scored
}

// calculateArticleScore scores articles based on trending potential
func calculateArticleScore(item models.RSSItem) int {
	score := 50 // base score

	title := strings.ToLower(item.Title)
	source := strings.ToLower(item.Source)

	// High trending keywords boost score
	trendingKeywords := []string{
		"trump", "iran", "war", "breaking", "major",
		"election", "climate", "economy", "stock",
		"tech", "ai", "apple", "google", "microsoft",
		"sports", "championship", "final", "win",
		"climate", "weather", "disaster", "emergency",
		" tariffs", "trade", "business", "company",
	}

	breakingKeywords := []string{
		"breaking", "urgent", "live", "developing",
		"just in", "announcement", "revealed", "confirmed",
	}

	for _, kw := range trendingKeywords {
		if strings.Contains(title, kw) {
			score += 15
		}
	}

	for _, kw := range breakingKeywords {
		if strings.Contains(title, kw) {
			score += 20
		}
	}

	// BBC and major sources get boost
	if strings.Contains(source, "bbc") || strings.Contains(source, "reuters") {
		score += 10
	}

	// Longer descriptions usually mean more substantial articles
	if len(item.Description) > 200 {
		score += 10
	}

	// Title length check - not too short or too long
	if len(title) > 30 && len(title) < 100 {
		score += 5
	}

	return score
}

func validateConfig(cfg *config.Config) error {
	if cfg.APIBaseURL == "" {
		return fmt.Errorf("API_BASE_URL is required")
	}
	if cfg.ReporterEmail == "" {
		return fmt.Errorf("REPORTER_EMAIL is required")
	}
	if cfg.ReporterPass == "" {
		return fmt.Errorf("REPORTER_PASSWORD is required")
	}
	return nil
}

func boolToYesNo(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
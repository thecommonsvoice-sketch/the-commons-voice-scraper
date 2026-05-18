package fetcher

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/mmcdole/gofeed"
	"scraper/internal/models"
)

type RSSSource struct {
	Name     string
	URL      string
	Category string
}

var RSSSources = []RSSSource{
	{Name: "BBC World", URL: "https://feeds.bbci.co.uk/news/world/rss.xml", Category: "general"},
	{Name: "BBC Tech", URL: "https://feeds.bbci.co.uk/news/technology/rss.xml", Category: "science-and-technology"},
	{Name: "BBC Business", URL: "https://feeds.bbci.co.uk/news/business/rss.xml", Category: "business"},
	{Name: "BBC Politics", URL: "https://feeds.bbci.co.uk/news/politics/rss.xml", Category: "politics"},
	{Name: "ESPN", URL: "https://www.espn.com/espn/rss/news", Category: "sports-and-entertainment"},
	{Name: "Al Jazeera", URL: "https://www.aljazeera.com/xml/rss/all.xml", Category: "world"},
}

type Fetcher struct {
	client *gofeed.Parser
}

func NewFetcher() *Fetcher {
	return &Fetcher{
		client: gofeed.NewParser(),
	}
}

func (f *Fetcher) FetchAll() []models.RSSItem {
	var allItems []models.RSSItem

	for _, source := range RSSSources {
		items, err := f.FetchFromSource(source)
		if err != nil {
			log.Printf("Error fetching from %s: %v", source.Name, err)
			continue
		}
		allItems = append(allItems, items...)
		log.Printf("Fetched %d items from %s", len(items), source.Name)
	}

	return allItems
}

func (f *Fetcher) FetchFromSource(source RSSSource) ([]models.RSSItem, error) {
	feed, err := f.client.ParseURL(source.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse feed: %w", err)
	}

	var items []models.RSSItem
	for _, item := range feed.Items {
		if item.Title == "" {
			continue
		}

		// Get image from RSS content/description
		imageURL := extractImageFromRSS(item)

		rssItem := models.RSSItem{
			Title:       cleanText(item.Title),
			Link:        item.Link,
			Description: cleanText(extractDescription(item)),
			PubDate:     item.Published,
			ImageURL:    imageURL,
			Source:      source.Name,
		}
		items = append(items, rssItem)
	}
	
	log.Printf("Fetched %d items from %s, images found: %d", len(items), source.Name, countImages(items))

	return items, nil
}

func countImages(items []models.RSSItem) int {
	count := 0
	for _, item := range items {
		if item.ImageURL != "" {
			count++
		}
	}
	return count
}

func extractImageFromRSS(item *gofeed.Item) string {
	// Try content first - extract ALL images and pick the largest
	if item.Content != "" {
		imgURLs := extractAllImagesFromHTML(item.Content)
		for _, imgURL := range imgURLs {
			if isValidImageURL(imgURL) && !isSmallImage(imgURL) {
				return imgURL
			}
		}
	}

	// Try description
	if item.Description != "" {
		imgURLs := extractAllImagesFromHTML(item.Description)
		for _, imgURL := range imgURLs {
			if isValidImageURL(imgURL) && !isSmallImage(imgURL) {
				return imgURL
			}
		}
	}

	// Try to find og:image meta tag
	if item.Content != "" {
		if imgURL := extractMeta(item.Content, "og:image"); imgURL != "" && isValidImageURL(imgURL) {
			return imgURL
		}
	}
	if item.Description != "" {
		if imgURL := extractMeta(item.Description, "og:image"); imgURL != "" && isValidImageURL(imgURL) {
			return imgURL
		}
	}

	return ""
}

// extractAllImagesFromHTML extracts ALL image URLs from HTML
func extractAllImagesFromHTML(html string) []string {
	var urls []string
	re := regexp.MustCompile(`<img[^>]+src=["']([^"']+)["']`)
	matches := re.FindAllStringSubmatch(html, -1)
	for _, match := range matches {
		if len(match) > 1 {
			urls = append(urls, match[1])
		}
	}
	return urls
}

// isSmallImage returns true if the URL likely points to a small/icon image
func isSmallImage(url string) bool {
	lowercaseURL := strings.ToLower(url)
	smallIndicators := []string{"icon", "logo", "button", "avatar", "thumb", "pixel", "1x1", "badge", "spacer", "blank"}
	for _, indicator := range smallIndicators {
		if strings.Contains(lowercaseURL, indicator) {
			return true
		}
	}
	// Also skip very short URLs (likely tracking pixels)
	if len(url) < 50 {
		return true
	}
	return false
}

func extractMeta(html, name string) string {
	// Try property attribute
	re := regexp.MustCompile(fmt.Sprintf(`<meta[^>]+property=["']%s["'][^>]+content=["']([^"']+)["']`, name))
	matches := re.FindStringSubmatch(html)
	if len(matches) > 1 {
		return matches[1]
	}

	// Try name attribute
	re2 := regexp.MustCompile(fmt.Sprintf(`<meta[^>]+name=["']%s["'][^>]+content=["']([^"']+)["']`, name))
	matches2 := re2.FindStringSubmatch(html)
	if len(matches2) > 1 {
		return matches2[1]
	}

	// Try reversed order
	re3 := regexp.MustCompile(fmt.Sprintf(`<meta[^>]+content=["']([^"']+)["'][^>]+property=["']%s["']`, name))
	matches3 := re3.FindStringSubmatch(html)
	if len(matches3) > 1 {
		return matches3[1]
	}

	return ""
}

func extractSchemaImage(html string) string {
	re := regexp.MustCompile(`"image":\s*"([^"]+)"`)
	matches := re.FindStringSubmatch(html)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func extractFirstLargeImg(html string) string {
	re := regexp.MustCompile(`<img[^>]+src=["']([^"']+)["']`)
	matches := re.FindAllStringSubmatch(html, -1)

	for _, match := range matches {
		if len(match) > 1 {
			url := match[1]
			// Skip small images
			if !strings.Contains(url, "icon") &&
			   !strings.Contains(url, "logo") &&
			   !strings.Contains(url, "button") &&
			   !strings.Contains(url, "avatar") &&
			   !strings.Contains(url, "thumb") &&
			   !strings.Contains(url, "pixel") &&
			   !strings.Contains(url, "1x1") {
				return url
			}
		}
	}
	return ""
}

func extractImgFromHTML(html string) string {
	re := regexp.MustCompile(`<img[^>]+src=["']([^"']+)["']`)
	matches := re.FindStringSubmatch(html)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func isValidImageURL(url string) bool {
	if url == "" {
		return false
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return false
	}
	if strings.HasPrefix(url, "data:") {
		return false
	}
	return true
}

func extractDescription(item *gofeed.Item) string {
	if item.Description != "" {
		return item.Description
	}
	if item.Content != "" {
		return item.Content
	}
	return ""
}

func cleanText(text string) string {
	text = strings.TrimSpace(text)

	replacements := map[string]string{
		"&amp;":  "&",
		"&lt;":   "<",
		"&gt;":   ">",
		"&quot;": `"`,
		"&#39;":  "'",
		"&nbsp;": " ",
	}

	for old, new := range replacements {
		text = strings.ReplaceAll(text, old, new)
	}

	text = strings.ReplaceAll(text, "<p>", "")
	text = strings.ReplaceAll(text, "</p>", "")
	text = strings.ReplaceAll(text, "<br>", "")
	text = strings.ReplaceAll(text, "<br/>", "")

	return text
}
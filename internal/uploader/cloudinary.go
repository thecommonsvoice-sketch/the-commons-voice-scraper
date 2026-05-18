package uploader

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// isValidImageURL checks if URL is a valid image URL
func isValidImageURL(url string) bool {
	if url == "" {
		return false
	}
	// Must be http/https
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return false
	}
	// Must be an image extension
	imageExts := []string{".jpg", ".jpeg", ".png", ".gif", ".webp"}
	lowerURL := strings.ToLower(url)
	for _, ext := range imageExts {
		if strings.Contains(lowerURL, ext) {
			return true
		}
	}
	return false
}

type Uploader struct {
	cloudName      string
	apiKey         string
	apiSecret      string
	uploadPreset   string
	pixabayKey     string
	pexelsKey      string
	httpClient     *http.Client
}

func NewUploader(cloudinaryURL, uploadPreset string) *Uploader {
	cloudName, apiKey, apiSecret := parseCloudinaryURL(cloudinaryURL)

	return &Uploader{
		cloudName:    cloudName,
		apiKey:       apiKey,
		apiSecret:    apiSecret,
		uploadPreset: uploadPreset,
		pixabayKey:    "",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SetPixabayKey sets the Pixabay API key for image search
func (u *Uploader) SetPixabayKey(key string) {
	u.pixabayKey = key
}

// SetPexelsKey sets the Pexels API key for image search
func (u *Uploader) SetPexelsKey(key string) {
	u.pexelsKey = key
}

func parseCloudinaryURL(url string) (cloudName, apiKey, apiSecret string) {
	if url == "" {
		return "", "", ""
	}

	if strings.HasPrefix(url, "cloudinary://") {
		url = strings.TrimPrefix(url, "cloudinary://")
	}

	parts := strings.Split(url, "@")
	if len(parts) == 2 {
		creds := strings.Split(parts[0], ":")
		if len(creds) >= 2 {
			apiKey = creds[0]
			apiSecret = creds[1]
		}
		cloudName = parts[1]
	}

	return cloudName, apiKey, apiSecret
}

// UploadFromURL uploads an image directly from a URL to Cloudinary
func (u *Uploader) UploadFromURL(imageURL, articleTitle string) string {
	if imageURL == "" {
		return ""
	}
	return u.uploadFromURL(imageURL, articleTitle)
}

// FetchImageFromArticle fetches the main image from an article URL
func (u *Uploader) FetchImageFromArticle(articleURL, articleTitle string) string {
	if articleURL == "" {
		return ""
	}

	log.Printf("Fetching article page: %s", articleURL)
	
	req, _ := http.NewRequest("GET", articleURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	
	resp, err := u.httpClient.Do(req)
	if err != nil {
		log.Printf("Failed to fetch article: %v", err)
		return ""
	}
	defer resp.Body.Close()

	log.Printf("Article fetch status: %d", resp.StatusCode)
	
	if resp.StatusCode != 200 {
		return ""
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}

	html := string(body)
	
	// Try og:image first (most reliable)
	re := regexp.MustCompile(`<meta[^>]+property=["']og:image["'][^>]+content=["']([^"']+)["']`)
	matches := re.FindStringSubmatch(html)
	if len(matches) > 1 && isValidImageURL(matches[1]) {
		log.Printf("Found og:image from article: %s", matches[1])
		return matches[1]
	}

	// Try alternate og:image format
	re2 := regexp.MustCompile(`<meta[^>]+content=["']([^"']+)["'][^>]+property=["']og:image["']`)
	matches2 := re2.FindStringSubmatch(html)
	if len(matches2) > 1 && isValidImageURL(matches2[1]) {
		log.Printf("Found og:image from article: %s", matches2[1])
		return matches2[1]
	}

	// Try Twitter card image
	re3 := regexp.MustCompile(`<meta[^>]+name=["']twitter:image["'][^>]+content=["']([^"']+)["']`)
	matches3 := re3.FindStringSubmatch(html)
	if len(matches3) > 1 && isValidImageURL(matches3[1]) {
		log.Printf("Found twitter:image from article: %s", matches3[1])
		return matches3[1]
	}
	
	log.Printf("No og:image found on article page")
	return ""
}

// SearchAndUploadImage searches for an image using keywords and uploads to Cloudinary
// Priority: 1) Fetch article page for og:image, 2) DuckDuckGo, 3) Wikipedia, 4) Picsum fallback
func (u *Uploader) SearchAndUploadImage(articleTitle string) string {
	// Extract keywords from title
	keywords := extractKeywords(articleTitle)
	if len(keywords) == 0 {
		keywords = []string{"news", "current events"}
	}

	// Try DuckDuckGo image search (free, no API key)
	imageURL := u.searchDuckDuckGo(keywords)
	if imageURL != "" {
		log.Printf("Found related image from DuckDuckGo for: %s", articleTitle)
		return u.uploadFromURL(imageURL, articleTitle)
	}

	// Try Wikipedia Commons (free, no API key needed)
	imageURL = u.searchWikipediaCommons(keywords)
	if imageURL != "" {
		log.Printf("Found related image from Wikipedia for: %s", articleTitle)
		return u.uploadFromURL(imageURL, articleTitle)
	}

	// Try Pixabay (with API key)
	if u.pixabayKey != "" {
		imageURL = u.searchPixabay(keywords)
		if imageURL != "" {
			log.Printf("Found related image from Pixabay for: %s", articleTitle)
			return u.uploadFromURL(imageURL, articleTitle)
		}
	}

	// Final fallback: use picsum
	seed := hashString(articleTitle)
	picsumURL := fmt.Sprintf("https://picsum.photos/seed/%d/800/600", seed)
	log.Printf("Using fallback image for: %s", articleTitle)
	return u.uploadFromURL(picsumURL, articleTitle)
}

// searchDuckDuckGo searches for images using DuckDuckGo
func (u *Uploader) searchDuckDuckGo(keywords []string) string {
	query := strings.Join(keywords, "+")
	url := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s+images", query)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return ""
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}

	html := string(body)
	
	// Find image results - DuckDuckGo HTML format
	re := regexp.MustCompile(`data-src=["']([^"']+)["']|src=["']([^"']+\.jpg[^"']*)["']`)
	matches := re.FindAllStringSubmatch(html, 20)
	
	var imageURLs []string
	for _, match := range matches {
		if len(match) > 1 && match[1] != "" {
			imageURLs = append(imageURLs, match[1])
		} else if len(match) > 2 && match[2] != "" {
			imageURLs = append(imageURLs, match[2])
		}
	}

	// Filter and return a valid image
	for _, url := range imageURLs {
		if isValidImageURL(url) && !strings.Contains(url, "icon") && !strings.Contains(url, "logo") {
			// Try to get larger version - replace _mini with _medium or remove size suffix
			largeURL := strings.Replace(url, "_mini", "_medium", 1)
			largeURL = strings.Replace(largeURL, "us/", "us/i/", 1)
			return largeURL
		}
	}

	return ""
}

// searchWikipediaCommons searches for images on Wikipedia Commons
func (u *Uploader) searchWikipediaCommons(keywords []string) string {
	query := strings.Join(keywords, "+")
	url := fmt.Sprintf("https://en.wikipedia.org/w/api.php?action=query&list=search&srsearch=%s&srnamespace=6&format=json&utf8=1&limit=5", query)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	
	resp, err := u.httpClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return ""
	}

	var result struct {
		Query struct {
			Search []struct {
				Title string `json:"title"`
			} `json:"search"`
		} `json:"query"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ""
	}

	if len(result.Query.Search) == 0 {
		return ""
	}

	// Get first file info
	fileTitle := result.Query.Search[0].Title
	fileURL := fmt.Sprintf("https://en.wikipedia.org/w/api.php?action=query&titles=%s&prop=imageinfo&iiprop=url&format=json", strings.ReplaceAll(fileTitle, " ", "%20"))

	req2, _ := http.NewRequest("GET", fileURL, nil)
	resp2, err := u.httpClient.Do(req2)
	if err != nil {
		return ""
	}
	defer resp2.Body.Close()

	var result2 struct {
		Pages map[string]struct {
			ImageInfo []struct {
				URL string `json:"url"`
			} `json:"imageinfo"`
		} `json:"query.pages"`
	}

	if err := json.NewDecoder(resp2.Body).Decode(&result2); err != nil {
		return ""
	}

	for _, page := range result2.Pages {
		if len(page.ImageInfo) > 0 {
			return page.ImageInfo[0].URL
		}
	}

	return ""
}

func extractKeywords(title string) []string {
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true,
		"but": true, "in": true, "on": true, "at": true, "to": true,
		"for": true, "of": true, "is": true, "are": true, "was": true,
		"were": true, "be": true, "been": true, "being": true, "have": true,
		"has": true, "had": true, "do": true, "does": true, "did": true,
		"will": true, "would": true, "could": true, "should": true, "may": true,
		"might": true, "must": true, "can": true, "this": true, "that": true,
		"says": true, "said": true, "new": true, "first": true, "one": true,
		"two": true, "three": true, "four": true, "five": true,
	}

	words := strings.Fields(strings.ToLower(title))
	var keywords []string

	for _, word := range words {
		word = strings.Trim(word, ".,!?\"'()[]")
		if len(word) > 2 && !stopWords[word] {
			keywords = append(keywords, word)
		}
	}

	// Return top 3 keywords
	if len(keywords) > 3 {
		keywords = keywords[:3]
	}

	return keywords
}

func (u *Uploader) searchPixabay(keywords []string) string {
	query := strings.Join(keywords, "%20")
	
	// Try with API key if available
	if u.pixabayKey != "" {
		url := fmt.Sprintf("https://pixabay.com/api/?key=%s&q=%s&image_type=photo&per_page=5&orientation=horizontal", u.pixabayKey, query)
		return u.fetchPixabayImage(url)
	}
	
	// Try without key (Pixabay allows some requests without key)
	url := fmt.Sprintf("https://pixabay.com/en/photos/?q=%s&order=popular", query)
	return u.scrapePixabayURL(url, keywords)
}

// fetchPixabayImage fetches from Pixabay API
func (u *Uploader) fetchPixabayImage(url string) string {
	req, _ := http.NewRequest("GET", url, nil)
	resp, err := u.httpClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return ""
	}

	var result struct {
		Hits []struct {
			LargeImageURL string `json:"largeImageURL"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ""
	}

	if len(result.Hits) > 0 {
		rand.Seed(time.Now().UnixNano())
		return result.Hits[rand.Intn(len(result.Hits))].LargeImageURL
	}

	return ""
}

// scrapePixabayURL tries to get image from Pixabay search page (no API key needed)
func (u *Uploader) scrapePixabayURL(url string, keywords []string) string {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	
	resp, err := u.httpClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return ""
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}

	html := string(body)
	
	// Try to find image URLs in the HTML
	re := regexp.MustCompile(`https://cdn\.pixabay\.com/photo/\d+/\d+/\d+/[^"']+\.jpg`)
	matches := re.FindAllString(html, 5)
	
	if len(matches) > 0 {
		rand.Seed(time.Now().UnixNano())
		return matches[rand.Intn(len(matches))]
	}

	return ""
}

func (u *Uploader) searchPexels(keywords []string) string {
	if u.pexelsKey == "" {
		return ""
	}
	
	query := strings.Join(keywords, " ")
	url := fmt.Sprintf("https://api.pexels.com/v1/search?query=%s&per_page=3", query)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", u.pexelsKey)

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return ""
	}

	var result struct {
		Photos []struct {
			Src struct {
				Large string `json:"large"`
			} `json:"src"`
		} `json:"photos"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ""
	}

	if len(result.Photos) > 0 {
		rand.Seed(time.Now().UnixNano())
		return result.Photos[rand.Intn(len(result.Photos))].Src.Large
	}

	return ""
}

// uploadFromURL downloads image from URL and uploads to Cloudinary
func (u *Uploader) uploadFromURL(imageURL, articleTitle string) string {
	if imageURL == "" {
		return ""
	}

	log.Printf("Downloading image from: %s", imageURL)

	tmpFile, err := u.downloadImage(imageURL)
	if err != nil {
		log.Printf("Failed to download image: %v", err)
		return ""
	}
	defer os.Remove(tmpFile)

	return u.uploadFile(tmpFile, articleTitle)
}

func (u *Uploader) downloadImage(url string) (string, error) {
	resp, err := u.httpClient.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("scraper_%d.jpg", time.Now().Unix()))

	out, err := os.Create(tmpFile)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to save image: %w", err)
	}

	return tmpFile, nil
}

func (u *Uploader) uploadFile(filePath, articleTitle string) string {
	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("Failed to open file: %v", err)
		return ""
	}
	defer file.Close()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		log.Printf("Failed to create form file: %v", err)
		return ""
	}

	_, err = io.Copy(part, file)
	if err != nil {
		log.Printf("Failed to copy file: %v", err)
		return ""
	}

	folder := "scraper-articles"
	timestamp := time.Now().Unix()
	safeTitle := sanitizeFilename(articleTitle)
	publicID := fmt.Sprintf("%s/%d_%s", folder, timestamp, safeTitle)

	writer.WriteField("public_id", publicID)
	writer.WriteField("upload_preset", u.uploadPreset)
	writer.Close()

	url := fmt.Sprintf("https://api.cloudinary.com/v1_1/%s/image/upload", u.cloudName)

	req, err := http.NewRequest("POST", url, &buf)
	if err != nil {
		log.Printf("Failed to create request: %v", err)
		return ""
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := u.httpClient.Do(req)
	if err != nil {
		log.Printf("Upload request failed: %v", err)
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("Upload failed with status %d: %s", resp.StatusCode, string(respBody))
		return ""
	}

	var result struct {
		SecureURL string `json:"secure_url"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("Failed to parse response: %v", err)
		return ""
	}

	log.Printf("Uploaded image to Cloudinary: %s", result.SecureURL)
	return result.SecureURL
}

func hashString(s string) int {
	hash := 0
	for i, c := range s {
		hash = hash*31 + int(c) + i
	}
	if hash < 0 {
		hash = -hash
	}
	return hash % 100000
}

func sanitizeFilename(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, "\\", "-")
	name = strings.ReplaceAll(name, ":", "")
	name = strings.ReplaceAll(name, "?", "")
	name = strings.ReplaceAll(name, "*", "")
	name = strings.ReplaceAll(name, "\"", "")
	name = strings.ReplaceAll(name, "<", "")
	name = strings.ReplaceAll(name, ">", "")

	if len(name) > 50 {
		name = name[:50]
	}

	return name
}
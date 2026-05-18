package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"

	"scraper/internal/config"
	"scraper/internal/models"
)

type APIClient struct {
	baseURL string
	httpClient *http.Client
}

func NewAPIClient(cfg *config.Config) *APIClient {
	jar, _ := cookiejar.New(nil)
	return &APIClient{
		baseURL: cfg.APIBaseURL,
		httpClient: &http.Client{
			Jar: jar,
		},
	}
}

func (c *APIClient) Login(email, password string) error {
	loginReq := models.LoginRequest{
		Email:    email,
		Password: password,
	}

	body, err := json.Marshal(loginReq)
	if err != nil {
		return fmt.Errorf("failed to marshal login request: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+"/api/auth/login", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create login request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("login failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	log.Printf("Login successful, cookies stored in jar")
	return nil
}

func (c *APIClient) GetCategories() ([]models.Category, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/api/categories", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create categories request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("categories request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get categories failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Success    bool             `json:"success"`
		Categories []models.Category `json:"categories"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode categories response: %w", err)
	}

	return result.Categories, nil
}

func (c *APIClient) GetExistingArticles() (map[string]bool, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/api/articles?limit=1000", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create articles request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("articles request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return make(map[string]bool), nil
	}

	var result struct {
		Success  bool             `json:"success"`
		Articles []models.Article `json:"articles"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return make(map[string]bool), nil
	}

	slugMap := make(map[string]bool)
	for _, article := range result.Articles {
		slugMap[article.Slug] = true
	}

	log.Printf("Found %d existing articles for dedup", len(slugMap))
	return slugMap, nil
}

func (c *APIClient) CreateArticle(article *models.CreateArticleRequest) (*models.Article, error) {
	body, err := json.Marshal(article)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal article request: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+"/api/articles", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create article request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("create article request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return nil, fmt.Errorf("create article failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var result models.CreateArticleResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to decode article response: %w", err)
	}

	log.Printf("Created article: %s (status: %s)", result.Article.Title, result.Article.ID)
	return &result.Article, nil
}
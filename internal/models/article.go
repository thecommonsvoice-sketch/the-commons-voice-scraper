package models

import "time"

type RSSItem struct {
	Title       string
	Link        string
	Description string
	PubDate     string
	ImageURL    string
	Source      string
}

type Category struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type ArticleData struct {
	Title            string
	Content          string
	CategoryID       string
	CoverImage       string
	MetaTitle        string
	MetaDescription  string
	Tags             []string
	Status           string
}

type CreateArticleRequest struct {
	Title            string   `json:"title"`
	Content          string   `json:"content"`
	CategoryID       string   `json:"categoryId"`
	CoverImage       string   `json:"coverImage,omitempty"`
	MetaTitle        string   `json:"metaTitle"`
	MetaDescription  string   `json:"metaDescription"`
	Tags             []string `json:"tags"`
	Status           string   `json:"status"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

type ArticlesResponse struct {
	Articles []Article `json:"articles"`
}

type Article struct {
	ID    string `json:"id"`
	Slug  string `json:"slug"`
	Title string `json:"title"`
}

type CategoriesResponse struct {
	Success    bool       `json:"success"`
	Categories []Category `json:"categories,omitempty"`
}

type CreateArticleResponse struct {
	Success bool    `json:"success"`
	Article Article `json:"article"`
	Message string  `json:"message,omitempty"`
}

type ScrapedURL struct {
	URL       string    `json:"url"`
	Slug      string    `json:"slug"`
	ScrapedAt time.Time `json:"scrapedAt"`
}
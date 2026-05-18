package summarizer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

type GroqClient struct {
	apiKey string
	client *http.Client
}

func NewGroqClient(apiKey string) *GroqClient {
	return &GroqClient{
		apiKey: apiKey,
		client: &http.Client{},
	}
}

type GroqRequest struct {
	Model       string        `json:"model"`
	Messages    []GroqMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
}

var SupportedModels = []string{
	"llama-3.3-70b-versatile",
	"llama-3.1-8b-instant",
	"mixtral-8x7b-32768",
}

type GroqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type GroqResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (g *GroqClient) GenerateSummary(title, description, source string) (string, error) {
	if g.apiKey == "" {
		log.Println("No Groq API key - using fallback summarization")
		return g.fallbackSummary(title, description, source), nil
	}

	summary, err := g.doGenerateSummary(title, description, source)
	if err == nil {
		return summary, nil
	}

	// Check if it's a daily token limit (won't recover quickly)
	if strings.Contains(err.Error(), "tokens per day") {
		log.Printf("Groq daily token limit reached - using fallback summary")
		return g.fallbackSummary(title, description, source), nil
	}

	// Retry once for other rate limits
	log.Printf("Groq error: %v, retrying once...", err)
	time.Sleep(2 * time.Second)
	
	summary, err = g.doGenerateSummary(title, description, source)
	if err != nil {
		return g.fallbackSummary(title, description, source), err
	}

	return summary, nil
}

func (g *GroqClient) doGenerateSummary(title, description, source string) (string, error) {
	prompt := g.buildPrompt(title, description, source)

	request := GroqRequest{
		Model: "llama-3.3-70b-versatile",
		Messages: []GroqMessage{
			{
				Role:    "system",
				Content: "You are a professional news journalist. Write original, engaging summaries of news stories in your own words. Never copy text from the source. Always write in third person, professional journalism style.",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Temperature: 0.7,
		MaxTokens:   500,
	}

	body, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.groq.com/openai/v1/chat/completions", bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+g.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("Groq API error: %s", string(respBody))
		return "", fmt.Errorf("Groq API returned status %d", resp.StatusCode)
	}

	var groqResp GroqResponse
	if err := json.NewDecoder(resp.Body).Decode(&groqResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(groqResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	summary := groqResp.Choices[0].Message.Content
	log.Printf("Generated AI summary for: %s", title)

	return formatAsHTML(summary), nil
}

func (g *GroqClient) buildPrompt(title, description, source string) string {
	return fmt.Sprintf(`Write a 300-400 word original news summary based on the following article information. 
Write it in professional journalism style, in your own words - do NOT copy any text from the original.

Article Title: %s

Article Description/Excerpt: %s

Source: %s

Write the summary now:`, title, description, source)
}

func (g *GroqClient) fallbackSummary(title, description, source string) string {
	summary := fmt.Sprintf(`<p><strong>%s</strong></p>
<p>%s</p>
<p>This story was originally reported by %s. The developments mark an important update in ongoing coverage of this topic.</p>
<p>For more details and the latest updates, readers are encouraged to follow the original source and stay tuned for continued coverage.</p>`,
		title,
		truncateText(description, 300),
		source,
	)

	return summary
}

func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}

func formatAsHTML(content string) string {
	lines := strings.Split(content, "\n")
	var htmlLines []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		line = strings.ReplaceAll(line, "&", "&amp;")
		line = strings.ReplaceAll(line, "<", "&lt;")
		line = strings.ReplaceAll(line, ">", "&gt;")

		htmlLines = append(htmlLines, "<p>"+line+"</p>")
	}

	return strings.Join(htmlLines, "\n")
}
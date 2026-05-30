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
				Content: "You are an expert analytical journalist. Write deep, insightful, and completely original news reports in your own words. Never copy text from the source. Structure your articles beautifully with semantic HTML tags: use <h3> for sections, <p> for paragraphs, and occasionally <strong> or list tags (<ul>, <li>) if needed. Always write in third person, professional journalism style, without any meta-talk or introductory conversational filler.",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Temperature: 0.7,
		MaxTokens:   800,
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
	log.Printf("Generated analytical AI article for: %s", title)

	return formatAsHTML(summary), nil
}

func (g *GroqClient) buildPrompt(title, description, source string) string {
	return fmt.Sprintf(`Write an original, engaging, and deeply informative analytical news article based on the following recent event. Do not write a simple summary; provide analytical depth, context, or broader implications of the development so that it provides immense standalone value to readers.

Article Title: %s
Description/Excerpt: %s
Original Source Reference: %s

Please adhere to the following premium journalism structure:
1. An engaging introduction paragraph explaining the event and its immediate significance.
2. A section starting with <h3>Key Context & Background</h3> detailing why this event is occurring and what historical or market forces led to it.
3. A section starting with <h3>Broader Implications & Future Impact</h3> exploring what this means for the industry, society, or region in the medium-to-long term.
4. Use standard <h3> tags for subheadings, <p> tags for body paragraphs. Do not output markdown style subheadings like "###". Use HTML directly.
5. Do not include any standard conversational introductory or concluding text (like "Here is the article..."). Start directly with the first paragraph.

Length: 450-650 words. Write the entire article in a premium, professional journalism style:`, title, description, source)
}

func (g *GroqClient) fallbackSummary(title, description, source string) string {
	summary := fmt.Sprintf(`<p><strong>%s</strong></p>
<p>%s</p>
<p>This development represents an important update in ongoing coverage. Industry analysts and observers are actively tracking the situation to assess its long-term significance.</p>
<p>As the situation continues to unfold, further updates and expert perspectives are expected to emerge from verified reporting channels.</p>`,
		title,
		truncateText(description, 300),
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

		// If the AI already structured it using HTML tags (like <h3>, <p>, <ul>, <li>), keep them directly!
		if strings.HasPrefix(line, "<h3") || strings.HasPrefix(line, "<p") || strings.HasPrefix(line, "<ul") || strings.HasPrefix(line, "<li") || strings.HasPrefix(line, "<strong") || strings.HasPrefix(line, "</") {
			htmlLines = append(htmlLines, line)
			continue
		}

		// Otherwise, escape and wrap in paragraph
		line = strings.ReplaceAll(line, "&", "&amp;")
		line = strings.ReplaceAll(line, "<", "&lt;")
		line = strings.ReplaceAll(line, ">", "&gt;")

		htmlLines = append(htmlLines, "<p>"+line+"</p>")
	}

	return strings.Join(htmlLines, "\n")
}
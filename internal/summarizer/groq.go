package summarizer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
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
		client: &http.Client{Timeout: 120 * time.Second},
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

func parseRetryAfter(body string) time.Duration {
	re := regexp.MustCompile(`Please try again in (\d+)ms`)
	m := re.FindStringSubmatch(body)
	if len(m) >= 2 {
		if ms, err := strconv.Atoi(m[1]); err == nil {
			return time.Duration(ms+500) * time.Millisecond
		}
	}
	return 0
}

func (g *GroqClient) GenerateSummary(title, description, source string) (string, error) {
	if g.apiKey == "" {
		log.Println("No Groq API key - cannot generate article")
		return "", fmt.Errorf("no GROQ_API_KEY configured")
	}

	prompt := g.buildPrompt(title, description, source)
	systemMsg := "You are an expert analytical journalist. Write deep, insightful, and completely original news reports in your own words. Never copy text from the source. Structure your articles beautifully with semantic HTML tags: use <h3> for sections, <p> for paragraphs, and occasionally <strong> or list tags (<ul>, <li>) if needed. Always write in third person, professional journalism style, without any meta-talk or introductory conversational filler."

	models := []string{"llama-3.3-70b-versatile", "llama-3.1-8b-instant"}
	maxRetries := 3

	var lastErr error
	for mi, model := range models {
		for attempt := 0; attempt < maxRetries; attempt++ {
			body, _ := json.Marshal(GroqRequest{
				Model: model,
				Messages: []GroqMessage{
					{Role: "system", Content: systemMsg},
					{Role: "user", Content: prompt},
				},
				Temperature: 0.7,
				MaxTokens:   1500,
			})

			req, err := http.NewRequest("POST", "https://api.groq.com/openai/v1/chat/completions", bytes.NewBuffer(body))
			if err != nil {
				lastErr = fmt.Errorf("create request: %w", err)
				continue
			}

			req.Header.Set("Authorization", "Bearer "+g.apiKey)
			req.Header.Set("Content-Type", "application/json")

			resp, err := g.client.Do(req)
			if err != nil {
				lastErr = fmt.Errorf("request failed: %w", err)
				time.Sleep(time.Duration(attempt+1) * time.Second)
				continue
			}

			if resp.StatusCode == 429 {
				b, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				wait := parseRetryAfter(string(b))
				if wait > 0 {
					log.Printf("  ⏳ Rate limited on %s, waiting %.1fs...", model, wait.Seconds())
					time.Sleep(wait)
				} else {
					time.Sleep(time.Duration(2*(attempt+1)) * time.Second)
				}
				lastErr = fmt.Errorf("rate limited: model=%s attempt=%d", model, attempt+1)
				continue
			}

			if resp.StatusCode != 200 {
				b, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				lastErr = fmt.Errorf("Groq API %d: %s", resp.StatusCode, string(b))
				time.Sleep(time.Second)
				continue
			}

			var gr GroqResponse
			if err := json.NewDecoder(resp.Body).Decode(&gr); err != nil {
				resp.Body.Close()
				lastErr = fmt.Errorf("decode: %w", err)
				continue
			}
			resp.Body.Close()

			if len(gr.Choices) == 0 {
				lastErr = fmt.Errorf("no choices")
				continue
			}

			log.Printf("Generated analytical AI article for: %s", title)
			return formatAsHTML(gr.Choices[0].Message.Content), nil
		}

		if mi < len(models)-1 {
			log.Printf("  ↳ %s exhausted, falling back to %s", model, models[mi+1])
		}
	}

	return "", fmt.Errorf("all models exhausted: %w", lastErr)
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

func formatAsHTML(content string) string {
	lines := strings.Split(content, "\n")
	var htmlLines []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "<h3") || strings.HasPrefix(line, "<p") || strings.HasPrefix(line, "<ul") || strings.HasPrefix(line, "<li") || strings.HasPrefix(line, "<strong") || strings.HasPrefix(line, "</") {
			htmlLines = append(htmlLines, line)
			continue
		}

		line = strings.ReplaceAll(line, "&", "&amp;")
		line = strings.ReplaceAll(line, "<", "&lt;")
		line = strings.ReplaceAll(line, ">", "&gt;")

		htmlLines = append(htmlLines, "<p>"+line+"</p>")
	}

	return strings.Join(htmlLines, "\n")
}
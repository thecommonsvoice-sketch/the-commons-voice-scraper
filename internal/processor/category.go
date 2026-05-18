package processor

import (
	"strings"

	"scraper/internal/models"
)

type CategoryMapper struct {
	slugToID map[string]string
}

func NewCategoryMapper(categories []models.Category) *CategoryMapper {
	mapper := &CategoryMapper{
		slugToID: make(map[string]string),
	}

	for _, cat := range categories {
		mapper.slugToID[cat.Slug] = cat.ID
	}

	return mapper
}

func (m *CategoryMapper) GetCategoryID(sourceName string) string {
	sourceCategoryMap := map[string]string{
		"BBC World":    "general",
		"BBC Tech":     "science-and-technology",
		"BBC Business": "business",
		"BBC Politics": "politics",
		"ESPN":         "sports-and-entertainment",
		"Al Jazeera":   "world",
	}

	slug := sourceCategoryMap[sourceName]
	if slug == "" {
		slug = "general"
	}

	return m.slugToID[slug]
}

func (m *CategoryMapper) GetCategoryIDFromSlug(slug string) string {
	if id, ok := m.slugToID[slug]; ok {
		return id
	}
	return m.slugToID["general"]
}

func GenerateSlug(title string) string {
	slug := strings.ToLower(title)

	replacements := map[string]string{
		" ":  "-",
		"?":  "",
		"!":  "",
		".":  "",
		",":  "",
		"'":  "",
		"\"": "",
		"(":  "",
		")":  "",
		"[":  "",
		"]":  "",
		"&":  "and",
		"@":  "at",
		"#":  "",
		"$":  "",
		"%":  "",
		"^":  "",
		"*":  "",
		"+":  "",
		"=":  "",
		":":  "",
		";":  "",
	}

	for old, new := range replacements {
		slug = strings.ReplaceAll(slug, old, new)
	}

	slug = strings.Trim(slug, "-")

	slug = strings.ReplaceAll(slug, "--", "-")

	if len(slug) > 100 {
		slug = slug[:100]
	}

	return slug
}

func ExtractKeywords(title string) []string {
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true,
		"but": true, "in": true, "on": true, "at": true, "to": true,
		"for": true, "of": true, "is": true, "are": true, "was": true,
		"were": true, "be": true, "been": true, "being": true, "have": true,
		"has": true, "had": true, "do": true, "does": true, "did": true,
		"will": true, "would": true, "could": true, "should": true, "may": true,
		"might": true, "must": true, "can": true, "this": true, "that": true,
	}

	words := strings.Fields(strings.ToLower(title))
	var keywords []string

	for _, word := range words {
		word = strings.Trim(word, ".,!?\"'()[]")
		if len(word) > 2 && !stopWords[word] {
			keywords = append(keywords, word)
		}
	}

	if len(keywords) > 5 {
		keywords = keywords[:5]
	}

	return keywords
}
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

// CategorizeByContent determines which navbar category an article belongs to
// based on its title, description, and source using keyword matching.
func CategorizeByContent(title, description, source string) string {
	text := strings.ToLower(title + " " + description + " " + source)

	// Category keyword maps — ordered by specificity
	categoryKeywords := []struct {
		slug     string
		keywords []string
	}{
		{
			"defence",
			[]string{"army", "military", "navy", "air force", "soldier", "troop", "weapon",
				"missile", "tank", "drone strike", "defence", "defense", "border security",
				"ammunition", "artillery", "submarine", "fighter jet", "naval",
				"defence ministry", "defense ministry", "general ", "general,",
				"armed forces", "militant", "insurgency", "counter-terrorism",
				"cyber attack", "cyber warfare", "nuclear weapon"},
		},
		{
			"politics",
			[]string{"election", "parliament", "president", "prime minister", "minister",
				"government", "politician", "senate", "congress", "democrat", "republican",
				"vote", "campaign", "political", "governor", "chancellor", "candidate",
				"legislation", "policy", "bill", "lawmaker", "mp", "mps", "ruling party",
				"opposition", "coalition", "diplomat", "diplomacy", "sanction",
				"ambassador", "treaty", "summit", "g20", "g7", "united nations"},
		},
		{
			"business",
			[]string{"stock", "market", "economy", "trade", "tariff", "company", "ceo",
				"business", "finance", "bank", "investment", "share", "profit", "revenue",
				"merger", "acquisition", "ipo", "startup", "entrepreneur", "inflation",
				"gdp", "interest rate", "federal reserve", "central bank", "crypto",
				"bitcoin", "blockchain", "wall street", "nasdaq", "dow jones", "s&p",
				"corporate", "industry", "manufacturing", "supply chain", "retail",
				"economic", "currency", "dollar", "fund", "investor", "portfolio"},
		},
		{
			"science-and-technology",
			[]string{"ai", "artificial intelligence", "tech", "technology", "software",
				"hardware", "apple", "google", "microsoft", "meta", "amazon", "tesla",
				"space", "nasa", "rocket", "satellite", "computer", "robot", "robotic",
				"data", "algorithm", "machine learning", "deep learning", "chatgpt",
				"openai", "llm", "neural", "quantum", "cyber", "cybersecurity",
				"smartphone", "iphone", "android", "chip", "semiconductor", "processor",
				"gpu", "5g", "internet", "digital", "app", "virtual reality", "vr",
				"augmented reality", "ar", "science", "research", "study", "discovery",
				"lab", "scientist", "experiment", "climate change", "global warming",
				"environment", "renewable", "solar", "wind energy", "electric vehicle",
				"ev", "battery", "innovation", "patent", "biotech", "gene", "dna",
				"spacex", "blue origin", "telescope", "mars", "moon", "asteroid"},
		},
		{
			"sports-and-entertainment",
			[]string{"sport", "game", "match", "tournament", "championship", "league",
				"player", "team", "coach", "goal", "score", "win", "final", "olympic",
				"cricket", "football", "soccer", "basketball", "tennis", "baseball",
				"f1", "formula one", "boxing", "ufc", "wrestling", "golf", "hockey",
				"rugby", "athlete", "champion", "medal", "world cup", "super bowl",
				"movie", "film", "actor", "actress", "celebrity", "music", "song",
				"album", "concert", "entertainment", "hollywood", "netflix", "tv show",
				"series", "oscar", "grammy", "award", "theatre", "artist", "dance",
				"star", "director", "producer", "box office", "streaming", "disney",
				"marvel", "dc", "game", "gaming", "esports", "playstation", "xbox",
				"nintendo", "twitch", "youtube", "tiktok", "instagram", "influencer",
				"viral", "trending"},
		},
		{
			"world",
			[]string{"international", "foreign", "global", "world", "china", "russia",
				"ukraine", "europe", "asia", "africa", "middle east", "latin america",
				"india", "japan", "germany", "france", "uk", "britain", "australia",
				"canada", "iran", "israel", "palestine", "gaza", "ukraine war", "moscow",
				"beijing", "washington", "london", "paris", "berlin", "tokyo", "seoul",
				"nato", "eu", "european union", "united nations", "refugee", "migrant",
				"humanitarian", "aid", "crisis", "conflict", "border", "embassy",
				"overseas", "diplomatic", "sanctions", "treaty", "alliance",
				"al jazeera", "bbc world", "reuters", "associated press", "ap"},
		},
	}

	// Score each category
	bestScore := 0
	bestSlug := "general"

	for _, cat := range categoryKeywords {
		score := 0
		for _, kw := range cat.keywords {
			if strings.Contains(text, kw) {
				score += 10
				// Extra weight if keyword is in the title
				if strings.Contains(strings.ToLower(title), kw) {
					score += 15
				}
			}
		}
		if score > bestScore {
			bestScore = score
			bestSlug = cat.slug
		}
	}

	return bestSlug
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
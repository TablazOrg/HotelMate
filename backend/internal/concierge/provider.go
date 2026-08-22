package concierge

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/google/uuid"
)

// Knowledge is an already approved and tenant-scoped answer. Providers never
// receive drafts or rejected content.
type Knowledge struct {
	ID       uuid.UUID
	Question string
	Answer   string
}

type Answer struct {
	Body        string
	Confidence  float64
	KnowledgeID *uuid.UUID
}

// Provider is the feature boundary for future external AI integrations. A
// provider may only answer from the approved knowledge passed by the caller.
type Provider interface {
	Answer(context.Context, string, []Knowledge) (Answer, error)
}

// ApprovedKnowledgeProvider is deterministic, auditable, and safe as the
// default production provider. It selects the closest approved answer without
// sending guest content to a third party.
type ApprovedKnowledgeProvider struct{}

func (ApprovedKnowledgeProvider) Answer(_ context.Context, query string, knowledge []Knowledge) (Answer, error) {
	queryTokens := tokens(query)
	if len(queryTokens) == 0 || len(knowledge) == 0 {
		return Answer{}, nil
	}
	type candidate struct {
		item  Knowledge
		score float64
	}
	candidates := make([]candidate, 0, len(knowledge))
	for _, item := range knowledge {
		answerTokens := tokens(item.Question + " " + item.Answer)
		matched := 0
		for token := range queryTokens {
			if _, ok := answerTokens[token]; ok {
				matched++
			}
		}
		score := float64(matched) / float64(len(queryTokens))
		if strings.Contains(normalize(item.Question+" "+item.Answer), normalize(query)) {
			score = 1
		}
		candidates = append(candidates, candidate{item: item, score: score})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score == candidates[j].score {
			return candidates[i].item.ID.String() < candidates[j].item.ID.String()
		}
		return candidates[i].score > candidates[j].score
	})
	best := candidates[0]
	if best.score == 0 {
		return Answer{}, nil
	}
	id := best.item.ID
	return Answer{Body: best.item.Answer, Confidence: best.score, KnowledgeID: &id}, nil
}

var injectionMarkers = []string{
	"ignore previous", "ignore all instructions", "system prompt", "developer message",
	"reveal instructions", "jailbreak", "prompt injection", "دستورالعمل قبلی", "دستورهای قبلی",
	"پیام سیستم", "پرامپت سیستم", "قوانینت را نادیده", "محدودیت را دور بزن",
}

func IsPromptInjection(body string) bool {
	normalized := normalize(body)
	for _, marker := range injectionMarkers {
		if strings.Contains(normalized, normalize(marker)) {
			return true
		}
	}
	return false
}

var (
	emailPattern      = regexp.MustCompile(`(?i)[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}`)
	longNumberPattern = regexp.MustCompile(`[0-9۰-۹٠-٩](?:[0-9۰-۹٠-٩ \-]{6,}[0-9۰-۹٠-٩])`)
	multiSpacePattern = regexp.MustCompile(`\s+`)
)

// SanitizeGuestText removes common direct identifiers before persistence.
// Short room numbers, dates, prices, and times remain usable for support.
func SanitizeGuestText(body string) (string, bool) {
	clean := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, strings.TrimSpace(body))
	redacted := false
	if emailPattern.MatchString(clean) {
		clean = emailPattern.ReplaceAllString(clean, "[اطلاعات شخصی حذف شد]")
		redacted = true
	}
	if longNumberPattern.MatchString(clean) {
		clean = longNumberPattern.ReplaceAllString(clean, "[اطلاعات شخصی حذف شد]")
		redacted = true
	}
	clean = strings.TrimSpace(multiSpacePattern.ReplaceAllString(clean, " "))
	return clean, redacted
}

var stopWords = map[string]struct{}{
	"از": {}, "به": {}, "در": {}, "را": {}, "با": {}, "برای": {}, "و": {}, "یا": {}, "که": {},
	"چه": {}, "چی": {}, "چیه": {}, "است": {}, "هست": {}, "هتل": {}, "لطفا": {}, "لطفاً": {},
	"تا": {}, "می": {}, "شود": {}, "میشه": {}, "does": {}, "the": {}, "is": {}, "a": {}, "an": {},
}

func tokens(value string) map[string]struct{} {
	result := make(map[string]struct{})
	for _, token := range strings.FieldsFunc(normalize(value), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) }) {
		token = stem(token)
		if len([]rune(token)) < 2 {
			continue
		}
		if _, skip := stopWords[token]; skip {
			continue
		}
		result[token] = struct{}{}
	}
	return result
}

func normalize(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer("ي", "ی", "ك", "ک", "ۀ", "ه", "ة", "ه", "‌", " ", "wifi", "وای فای")
	return multiSpacePattern.ReplaceAllString(replacer.Replace(value), " ")
}

func stem(token string) string {
	for _, suffix := range []string{"هایی", "های", "ترین", "تر", "ها", "ی", "ه"} {
		if strings.HasSuffix(token, suffix) && len([]rune(token)) > len([]rune(suffix))+2 {
			return strings.TrimSuffix(token, suffix)
		}
	}
	return token
}

var _ Provider = ApprovedKnowledgeProvider{}

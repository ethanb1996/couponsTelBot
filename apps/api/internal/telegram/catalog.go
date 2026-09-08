package telegram

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"
)

const maxOffers = 8

var urlPattern = regexp.MustCompile(`https?://[^\s<>()]+`)

var offerIntroPattern = regexp.MustCompile(`(?i)^(?:קופון חדש ל[־-]?|קופון ל[־-]?|מבצע ב[־-]?|דיל ב[־-]?)\s*`)

type Offer struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	Text            string    `json:"text"`
	SourceMessageID int       `json:"source_message_id"`
	PublishedAt     time.Time `json:"published_at"`
}

type Catalog struct {
	mu            sync.RWMutex
	path          string
	offers        []Offer
	menuMessageID int
}

type catalogFile struct {
	Offers        []Offer `json:"offers"`
	MenuMessageID int     `json:"menu_message_id,omitempty"`
}

func NewCatalog(path string) (*Catalog, error) {
	c := &Catalog{path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return c, nil
		}
		return nil, err
	}
	if len(data) > 0 {
		var state catalogFile
		if err := json.Unmarshal(data, &state); err == nil && state.Offers != nil {
			c.offers = state.Offers
			c.menuMessageID = state.MenuMessageID
		} else if err := json.Unmarshal(data, &c.offers); err != nil {
			return nil, err
		}
	}
	if len(c.offers) > maxOffers {
		c.offers = c.offers[:maxOffers]
	}
	return c, nil
}

func (c *Catalog) Add(text string, messageID int, publishedAt time.Time) (Offer, bool, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return Offer{}, false, nil
	}
	key := offerKey(text)
	offer := Offer{
		ID:              shortHash(key),
		Title:           offerTitle(text),
		Text:            text,
		SourceMessageID: messageID,
		PublishedAt:     publishedAt.UTC(),
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	filtered := make([]Offer, 0, maxOffers)
	for _, existing := range c.offers {
		if existing.ID != offer.ID && existing.SourceMessageID != messageID {
			filtered = append(filtered, existing)
		}
	}
	wasDuplicate := len(filtered) != len(c.offers)
	c.offers = append([]Offer{offer}, filtered...)
	if len(c.offers) > maxOffers {
		c.offers = c.offers[:maxOffers]
	}
	return offer, wasDuplicate, c.saveLocked()
}

func (c *Catalog) List() []Offer {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return append([]Offer(nil), c.offers...)
}

func (c *Catalog) Get(id string) (Offer, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, offer := range c.offers {
		if offer.ID == id {
			return offer, true
		}
	}
	return Offer{}, false
}

func (c *Catalog) MenuMessageID() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.menuMessageID
}

func (c *Catalog) SetMenuMessageID(messageID int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.menuMessageID = messageID
	return c.saveLocked()
}

func (c *Catalog) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(catalogFile{
		Offers:        c.offers,
		MenuMessageID: c.menuMessageID,
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.path, data, 0o600)
}

func offerKey(text string) string {
	content := normalizedOfferContent(text)
	if match := urlPattern.FindString(text); match != "" {
		match = strings.TrimRight(match, ".,;:!?)]}")
		if parsed, err := url.Parse(match); err == nil {
			parsed.RawQuery = ""
			parsed.Fragment = ""
			parsed.Host = strings.ToLower(parsed.Host)
			parsed.Path = strings.TrimRight(parsed.Path, "/")
			return "url:" + parsed.String() + "|content:" + content
		}
	}
	return "content:" + content
}

func normalizedOfferContent(text string) string {
	lines := offerLines(text)
	if len(lines) > 2 {
		lines = lines[:2]
	}
	return normalize(strings.Join(lines, " "))
}

func offerButtonLabel(text string) string {
	lines := offerLines(text)
	if len(lines) == 0 {
		return "Coupon offer"
	}
	restaurant := cleanRestaurant(lines[0])
	if restaurant == "" {
		restaurant = strings.TrimSpace(lines[0])
	}
	if len(lines) == 1 {
		return truncateRunesWithEllipsis(restaurant, 40)
	}
	essence := cleanOfferEssence(lines[1])
	if essence == "" {
		return truncateRunesWithEllipsis(restaurant, 40)
	}
	restaurant = truncateRunesWithEllipsis(restaurant, 18)
	remaining := 40 - len([]rune(restaurant)) - len([]rune(" · "))
	if remaining < 10 {
		remaining = 10
	}
	return restaurant + " · " + truncateRunesWithEllipsis(essence, remaining)
}

func offerLines(text string) []string {
	var lines []string
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || urlPattern.MatchString(line) {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func cleanRestaurant(line string) string {
	line = trimDecorations(line)
	line = offerIntroPattern.ReplaceAllString(line, "")
	return strings.TrimSpace(line)
}

func cleanOfferEssence(line string) string {
	line = trimDecorations(line)
	for _, separator := range []string{" — ", " - "} {
		if before, _, ok := strings.Cut(line, separator); ok {
			line = before
			break
		}
	}
	line = strings.TrimSpace(strings.TrimPrefix(line, "שובר בשווי "))
	line = strings.TrimSpace(strings.TrimSuffix(line, "בלבד"))
	return strings.TrimSpace(line)
}

func trimDecorations(value string) string {
	return strings.TrimFunc(strings.TrimSpace(value), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r) && r != '%' && r != '₪'
	})
}

func offerTitle(text string) string {
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return truncateRunes(line, 48)
		}
	}
	return "Coupon offer"
}

func truncateRunesWithEllipsis(value string, max int) string {
	if max <= 1 {
		return truncateRunes(value, max)
	}
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max-1]) + "…"
}

func normalize(value string) string {
	normalized := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			return unicode.ToLower(r)
		}
		if unicode.IsSpace(r) {
			return ' '
		}
		return -1
	}, strings.Join(strings.Fields(value), " "))
	return strings.Join(strings.Fields(normalized), " ")
}

func shortHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:6])
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit-1]) + "…"
}

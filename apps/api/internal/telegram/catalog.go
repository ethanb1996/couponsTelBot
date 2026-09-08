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
	if match := urlPattern.FindString(text); match != "" {
		match = strings.TrimRight(match, ".,;:!?)]}")
		if parsed, err := url.Parse(match); err == nil {
			parsed.RawQuery = ""
			parsed.Fragment = ""
			parsed.Host = strings.ToLower(parsed.Host)
			parsed.Path = strings.TrimRight(parsed.Path, "/")
			return "url:" + parsed.String()
		}
	}
	return "title:" + normalize(offerTitle(text))
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

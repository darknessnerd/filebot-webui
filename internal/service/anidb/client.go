package anidb

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/darknessnerd/filebot-webui/internal/domain"
	"github.com/darknessnerd/filebot-webui/internal/logger"
)

const (
	defaultBaseURL    = "http://api.anidb.net:9001/httpapi"
	defaultProtoVer   = "1"
	minRequestSpacing = 2 * time.Second
)

type Client struct {
	http      *http.Client
	log       logger.Logger
	baseURL   string
	client    string
	clientVer string
	protoVer  string

	pacingMu      sync.Mutex
	lastRequestAt time.Time

	cacheMu    sync.RWMutex
	cacheByAID map[int]*domain.AnimeMatch

	idxMu      sync.RWMutex
	titleIndex *titleIndex
}

func NewClient(clientName, clientVer, protoVer, baseURL string, log logger.Logger) *Client {
	cleanBaseURL := strings.TrimSpace(baseURL)
	if cleanBaseURL == "" {
		cleanBaseURL = defaultBaseURL
	}
	cleanProto := strings.TrimSpace(protoVer)
	if cleanProto == "" {
		cleanProto = defaultProtoVer
	}

	return &Client{
		http:       &http.Client{Timeout: 10 * time.Second},
		log:        log,
		baseURL:    cleanBaseURL,
		client:     strings.TrimSpace(clientName),
		clientVer:  strings.TrimSpace(clientVer),
		protoVer:   cleanProto,
		cacheByAID: make(map[int]*domain.AnimeMatch),
	}
}

// LoadIndexFromFile loads the title index from disk at the given path.
// Called once at startup when a titles file already exists on disk.
func (c *Client) LoadIndexFromFile(path string) error {
	idx, err := loadTitleIndexFromFile(strings.TrimSpace(path))
	if err != nil {
		return err
	}
	c.idxMu.Lock()
	c.titleIndex = idx
	c.idxMu.Unlock()
	return nil
}

// ReloadIndex parses new title XML bytes and atomically swaps the live index.
// Safe to call from the background scheduler while searches are in flight.
func (c *Client) ReloadIndex(data []byte) error {
	idx, err := loadTitleIndexFromBytes(data)
	if err != nil {
		return err
	}
	c.idxMu.Lock()
	c.titleIndex = idx
	c.idxMu.Unlock()
	c.log.Info().Int("titles", len(idx.byTitle)).Msg("anidb: title index reloaded")
	return nil
}

func (c *Client) SearchAnime(ctx context.Context, query string, year int) (*domain.AnimeMatch, error) {
	c.idxMu.RLock()
	idx := c.titleIndex
	c.idxMu.RUnlock()

	if idx == nil {
		return nil, fmt.Errorf("anidb.SearchAnime: title index not loaded; scheduler must run first or set ANIDB_REFRESH_TITLES_ON_START=true")
	}
	candidates, err := idx.FindCandidates(query)
	if err != nil {
		return nil, fmt.Errorf("anidb.SearchAnime: %w", err)
	}
	if len(candidates) == 1 {
		return c.SearchAnimeByAID(ctx, candidates[0])
	}
	if year <= 0 {
		return nil, fmt.Errorf("anidb.SearchAnime: ambiguous title %q (candidate aids: %v)", query, candidates)
	}

	var yearMatches []*domain.AnimeMatch
	for _, aid := range candidates {
		match, err := c.SearchAnimeByAID(ctx, aid)
		if err != nil {
			return nil, fmt.Errorf("anidb.SearchAnime: resolving candidate aid=%d for %q: %w", aid, query, err)
		}
		if match.Year == year {
			yearMatches = append(yearMatches, match)
		}
	}
	switch len(yearMatches) {
	case 1:
		return yearMatches[0], nil
	case 0:
		return nil, fmt.Errorf("anidb.SearchAnime: ambiguous title %q, no candidate matches year %d (candidate aids: %v)", query, year, candidates)
	default:
		return nil, fmt.Errorf("anidb.SearchAnime: ambiguous title %q, multiple candidates match year %d (candidate aids: %v)", query, year, candidates)
	}
}

func (c *Client) SearchAnimeByAID(ctx context.Context, aid int) (*domain.AnimeMatch, error) {
	if c.client == "" || c.clientVer == "" {
		return nil, fmt.Errorf("anidb.SearchAnimeByAID: client and clientver are required")
	}
	if aid <= 0 {
		return nil, fmt.Errorf("anidb.SearchAnimeByAID: invalid aid %d", aid)
	}

	if cached, ok := c.cachedMatch(aid); ok {
		return cached, nil
	}

	if err := c.waitForPacing(ctx); err != nil {
		return nil, fmt.Errorf("anidb.SearchAnimeByAID: %w", err)
	}

	endpoint, err := c.animeURL(aid)
	if err != nil {
		return nil, fmt.Errorf("anidb.SearchAnimeByAID url: %w", err)
	}

	c.log.Debug().Int("aid", aid).Str("endpoint", endpoint).Msg("anidb: searching anime by aid")

	body, err := c.get(ctx, endpoint)
	if err != nil {
		return nil, fmt.Errorf("anidb.SearchAnimeByAID: %w", err)
	}

	var apiErr apiError
	if err := xml.Unmarshal(body, &apiErr); err == nil && apiErr.XMLName.Local == "error" {
		msg := strings.TrimSpace(apiErr.Message)
		if msg == "" {
			msg = "unknown AniDB error"
		}
		return nil, fmt.Errorf("anidb.SearchAnimeByAID: code %s: %s", strings.TrimSpace(apiErr.Code), msg)
	}

	var anime animeResponse
	if err := xml.Unmarshal(body, &anime); err != nil {
		return nil, fmt.Errorf("decode anime xml: %w", err)
	}
	if anime.XMLName.Local != "anime" {
		return nil, fmt.Errorf("unexpected root element %q", anime.XMLName.Local)
	}
	if anime.ID == 0 {
		return nil, fmt.Errorf("no anime id in response for aid=%d", aid)
	}

	title := pickTitle(anime.Titles)
	if title == "" {
		return nil, fmt.Errorf("no anime title in response for aid=%d", aid)
	}

	matchYear := yearFromDate(anime.StartDate)
	match := &domain.AnimeMatch{
		ID:    anime.ID,
		Title: title,
		Year:  matchYear,
	}
	c.storeCache(match)
	return match, nil
}

func (c *Client) animeURL(aid int) (string, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("request", "anime")
	q.Set("client", c.client)
	q.Set("clientver", c.clientVer)
	q.Set("protover", c.protoVer)
	q.Set("aid", strconv.Itoa(aid))
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (c *Client) waitForPacing(ctx context.Context) error {
	c.pacingMu.Lock()
	wait := time.Until(c.lastRequestAt.Add(minRequestSpacing))
	if wait <= 0 {
		c.lastRequestAt = time.Now()
		c.pacingMu.Unlock()
		return nil
	}
	c.pacingMu.Unlock()

	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return fmt.Errorf("request pacing interrupted: %w", ctx.Err())
	case <-timer.C:
	}

	c.pacingMu.Lock()
	c.lastRequestAt = time.Now()
	c.pacingMu.Unlock()
	return nil
}

func (c *Client) cachedMatch(aid int) (*domain.AnimeMatch, bool) {
	c.cacheMu.RLock()
	defer c.cacheMu.RUnlock()
	match, ok := c.cacheByAID[aid]
	if !ok || match == nil {
		return nil, false
	}
	copy := *match
	return &copy, true
}

func (c *Client) storeCache(match *domain.AnimeMatch) {
	if match == nil || match.ID <= 0 {
		return
	}
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	cp := *match
	c.cacheByAID[match.ID] = &cp
}

func (c *Client) get(ctx context.Context, endpoint string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do: %w", err)
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if readErr != nil {
		return nil, fmt.Errorf("read body: %w", readErr)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}

func pickTitle(titles []animeTitle) string {
	for _, t := range titles {
		if strings.EqualFold(strings.TrimSpace(t.Type), "main") && strings.TrimSpace(t.Value) != "" {
			return strings.TrimSpace(t.Value)
		}
	}
	for _, t := range titles {
		if strings.EqualFold(strings.TrimSpace(t.Type), "official") && strings.TrimSpace(t.Value) != "" {
			return strings.TrimSpace(t.Value)
		}
	}
	for _, t := range titles {
		if strings.TrimSpace(t.Value) != "" {
			return strings.TrimSpace(t.Value)
		}
	}
	return ""
}

func yearFromDate(s string) int {
	if len(s) < 4 {
		return 0
	}
	year, err := strconv.Atoi(s[:4])
	if err != nil {
		return 0
	}
	return year
}

type animeResponse struct {
	XMLName   xml.Name     `xml:"anime"`
	ID        int          `xml:"id,attr"`
	StartDate string       `xml:"startdate"`
	Titles    []animeTitle `xml:"titles>title"`
}

type animeTitle struct {
	Type  string `xml:"type,attr"`
	Value string `xml:",chardata"`
}

type apiError struct {
	XMLName xml.Name `xml:"error"`
	Code    string   `xml:"code,attr"`
	Message string   `xml:",chardata"`
}

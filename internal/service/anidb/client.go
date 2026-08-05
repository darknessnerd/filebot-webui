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

	mu            sync.Mutex
	lastRequestAt time.Time
	cacheByAID    map[int]*domain.AnimeMatch
	titleIndex    *titleIndex
	titleIndexErr error
}

func NewClient(clientName, clientVer, protoVer, baseURL, titlesFile string, log logger.Logger) *Client {
	cleanBaseURL := strings.TrimSpace(baseURL)
	if cleanBaseURL == "" {
		cleanBaseURL = defaultBaseURL
	}
	cleanProto := strings.TrimSpace(protoVer)
	if cleanProto == "" {
		cleanProto = defaultProtoVer
	}

	c := &Client{
		http:       &http.Client{Timeout: 10 * time.Second},
		log:        log,
		baseURL:    cleanBaseURL,
		client:     strings.TrimSpace(clientName),
		clientVer:  strings.TrimSpace(clientVer),
		protoVer:   cleanProto,
		cacheByAID: make(map[int]*domain.AnimeMatch),
	}
	if strings.TrimSpace(titlesFile) != "" {
		idx, err := loadTitleIndexFromFile(strings.TrimSpace(titlesFile))
		if err != nil {
			c.titleIndexErr = err
			c.log.Warn().Err(err).Str("titles_file", titlesFile).Msg("anidb: failed to load title index")
		} else {
			c.titleIndex = idx
		}
	}
	return c
}

func (c *Client) SearchAnime(ctx context.Context, query string, _ int) (*domain.AnimeMatch, error) {
	if c.titleIndexErr != nil {
		return nil, fmt.Errorf("anidb.SearchAnime: title index unavailable: %w", c.titleIndexErr)
	}
	if c.titleIndex == nil {
		return nil, fmt.Errorf("anidb.SearchAnime: title index not configured; set ANIDB_TITLES_FILE or use --q aid:<id>")
	}
	aid, err := c.titleIndex.FindAID(query)
	if err != nil {
		return nil, fmt.Errorf("anidb.SearchAnime: %w", err)
	}
	return c.SearchAnimeByAID(ctx, aid)
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
	c.mu.Lock()
	wait := time.Until(c.lastRequestAt.Add(minRequestSpacing))
	if wait <= 0 {
		c.lastRequestAt = time.Now()
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return fmt.Errorf("request pacing interrupted: %w", ctx.Err())
	case <-timer.C:
	}

	c.mu.Lock()
	c.lastRequestAt = time.Now()
	c.mu.Unlock()
	return nil
}

func (c *Client) cachedMatch(aid int) (*domain.AnimeMatch, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
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
	c.mu.Lock()
	defer c.mu.Unlock()
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

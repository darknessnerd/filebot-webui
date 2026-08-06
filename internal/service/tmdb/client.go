package tmdb

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/darknessnerd/filebot-webui/internal/domain"
	"github.com/darknessnerd/filebot-webui/internal/logger"
)

type Client struct {
	http    *http.Client
	log     logger.Logger
	baseURL string
	token   string
}

func NewClient(token string, log logger.Logger) *Client {
	return &Client{
		http:    &http.Client{Timeout: 10 * time.Second},
		log:     log,
		baseURL: "https://api.themoviedb.org/3",
		token:   token,
	}
}

func (c *Client) SearchMovie(ctx context.Context, query string, year int) (*domain.MovieMatch, error) {
	endpoint, err := c.searchURL("/search/movie", query, year, "primary_release_year")
	if err != nil {
		return nil, fmt.Errorf("tmdb.SearchMovie url: %w", err)
	}

	c.log.Debug().Str("query", query).Int("year", year).Str("endpoint", endpoint).Msg("tmdb: searching movie")

	var body struct {
		Results []struct {
			ID          int     `json:"id"`
			Title       string  `json:"title"`
			ReleaseDate string  `json:"release_date"`
			Popularity  float64 `json:"popularity"`
		} `json:"results"`
	}
	if err := c.get(ctx, endpoint, &body); err != nil {
		return nil, fmt.Errorf("tmdb.SearchMovie: %w", err)
	}
	if len(body.Results) == 0 {
		return nil, fmt.Errorf("tmdb.SearchMovie: no results for %q", query)
	}

	best := body.Results[0]
	c.log.Debug().Int("id", best.ID).Str("title", best.Title).Str("release_date", best.ReleaseDate).Float64("popularity", best.Popularity).Int("total_results", len(body.Results)).Msg("tmdb: movie result selected")

	return &domain.MovieMatch{
		ID:    best.ID,
		Title: best.Title,
		Year:  yearFromDate(best.ReleaseDate),
	}, nil
}

func (c *Client) SearchTV(ctx context.Context, query string, year int) (*domain.TVMatch, error) {
	// first_air_date_year is the show's premiere year, not the season year.
	// A year in the filename (e.g. "Rick and Morty - Stagione 09 (2026)") refers to
	// the current season, not the pilot — always retry without year filter on empty results.
	type tvResult struct {
		Results []struct {
			ID           int     `json:"id"`
			Name         string  `json:"name"`
			FirstAirDate string  `json:"first_air_date"`
			Popularity   float64 `json:"popularity"`
		} `json:"results"`
	}

	search := func(y int) (*tvResult, error) {
		endpoint, err := c.searchURL("/search/tv", query, y, "first_air_date_year")
		if err != nil {
			return nil, fmt.Errorf("tmdb.SearchTV url: %w", err)
		}
		c.log.Debug().Str("query", query).Int("year", y).Str("endpoint", endpoint).Msg("tmdb: searching tv")
		var body tvResult
		if err := c.get(ctx, endpoint, &body); err != nil {
			return nil, fmt.Errorf("tmdb.SearchTV: %w", err)
		}
		return &body, nil
	}

	body, err := search(year)
	if err != nil {
		return nil, err
	}
	if len(body.Results) == 0 && year > 0 {
		c.log.Debug().Str("query", query).Int("year", year).Msg("tmdb: tv search with year returned no results, retrying without year")
		body, err = search(0)
		if err != nil {
			return nil, err
		}
	}
	if len(body.Results) == 0 {
		return nil, fmt.Errorf("tmdb.SearchTV: no results for %q", query)
	}

	best := body.Results[0]
	c.log.Debug().Int("id", best.ID).Str("name", best.Name).Str("first_air_date", best.FirstAirDate).Float64("popularity", best.Popularity).Int("total_results", len(body.Results)).Msg("tmdb: tv result selected")

	return &domain.TVMatch{
		ID:   best.ID,
		Name: best.Name,
		Year: yearFromDate(best.FirstAirDate),
	}, nil
}

func (c *Client) searchURL(path, query string, year int, yearKey string) (string, error) {
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("query", query)
	q.Set("include_adult", "false")
	q.Set("language", "en-US")
	q.Set("page", "1")
	if year > 0 {
		q.Set(yearKey, strconv.Itoa(year))
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (c *Client) get(ctx context.Context, endpoint string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	return nil
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

package filebot

import (
	"context"
	"fmt"

	"github.com/darknessnerd/filebot-webui/internal/domain"
)

type animeResolver interface {
	SearchAnime(ctx context.Context, query string, year int) (*domain.AnimeMatch, error)
	SearchAnimeByAID(ctx context.Context, aid int) (*domain.AnimeMatch, error)
}

type resolverSet struct {
	movie movieResolver
	tv    tvResolver
	anime animeResolver
}

func NewResolver(movie movieResolver, tv tvResolver, anime animeResolver) *resolverSet {
	return &resolverSet{movie: movie, tv: tv, anime: anime}
}

func (r *resolverSet) SearchMovie(ctx context.Context, query string, year int) (*domain.MovieMatch, error) {
	if r.movie == nil {
		return nil, fmt.Errorf("filebot.SearchMovie: resolver not configured")
	}
	return r.movie.SearchMovie(ctx, query, year)
}

func (r *resolverSet) SearchTV(ctx context.Context, query string, year int) (*domain.TVMatch, error) {
	if r.tv == nil {
		return nil, fmt.Errorf("filebot.SearchTV: resolver not configured")
	}
	return r.tv.SearchTV(ctx, query, year)
}

func (r *resolverSet) SearchAnimeByAID(ctx context.Context, aid int) (*domain.AnimeMatch, error) {
	if r.anime == nil {
		return nil, fmt.Errorf("filebot.SearchAnimeByAID: resolver not configured")
	}
	return r.anime.SearchAnimeByAID(ctx, aid)
}

func (r *resolverSet) SearchAnime(ctx context.Context, query string, year int) (*domain.AnimeMatch, error) {
	if r.anime == nil {
		return nil, fmt.Errorf("filebot.SearchAnime: resolver not configured")
	}
	return r.anime.SearchAnime(ctx, query, year)
}

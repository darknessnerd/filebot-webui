package anidb

import (
	"encoding/xml"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var titleNormPattern = regexp.MustCompile(`[^a-z0-9]+`)

type titleRef struct {
	AID  int
	Type string
}

type titleIndex struct {
	byTitle map[string][]titleRef
}

func loadTitleIndexFromFile(path string) (*titleIndex, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read titles file: %w", err)
	}
	return loadTitleIndexFromBytes(data)
}

func loadTitleIndexFromBytes(data []byte) (*titleIndex, error) {
	var doc animeTitlesDoc
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("decode titles xml: %w", err)
	}

	idx := &titleIndex{byTitle: map[string][]titleRef{}}
	for _, anime := range doc.Anime {
		aid, err := strconv.Atoi(strings.TrimSpace(anime.AID))
		if err != nil || aid <= 0 {
			continue
		}
		for _, t := range anime.Titles {
			if t.Type == "short" || t.Type == "kana" {
				continue
			}
			key := normalizeLookupTitle(t.Value)
			if key == "" {
				continue
			}
			idx.byTitle[key] = append(idx.byTitle[key], titleRef{AID: aid, Type: t.Type})
		}
	}

	return idx, nil
}

// titleTypeRank ranks AniDB title types so FindCandidates can prefer a
// "main" title match over a "synonym"/"short" one sharing the same text.
func titleTypeRank(t string) int {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "main":
		return 0
	case "official":
		return 1
	default:
		return 2
	}
}

// FindCandidates returns the aid(s) matching query, narrowed to the
// best-ranked title type present (main > official > other) so a "main"
// title on one anime beats a "synonym" collision on another. Multiple aids
// are returned when the tie survives at the best rank.
func (i *titleIndex) FindCandidates(query string) ([]int, error) {
	if i == nil {
		return nil, fmt.Errorf("title index not configured")
	}
	key := normalizeLookupTitle(query)
	if key == "" {
		return nil, fmt.Errorf("empty lookup title")
	}
	refs := i.byTitle[key]
	if len(refs) == 0 {
		return nil, fmt.Errorf("no aid found for %q", query)
	}

	bestRank := map[int]int{}
	for _, ref := range refs {
		rank := titleTypeRank(ref.Type)
		if cur, ok := bestRank[ref.AID]; !ok || rank < cur {
			bestRank[ref.AID] = rank
		}
	}

	minRank := 2
	for _, rank := range bestRank {
		if rank < minRank {
			minRank = rank
		}
	}

	var candidates []int
	for aid, rank := range bestRank {
		if rank == minRank {
			candidates = append(candidates, aid)
		}
	}
	sort.Ints(candidates)
	return candidates, nil
}

func (i *titleIndex) FindAID(query string) (int, error) {
	candidates, err := i.FindCandidates(query)
	if err != nil {
		return 0, err
	}
	if len(candidates) > 1 {
		return 0, fmt.Errorf("ambiguous title %q (multiple aids: %v)", query, candidates)
	}
	return candidates[0], nil
}

func normalizeLookupTitle(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = titleNormPattern.ReplaceAllString(s, " ")
	return strings.Join(strings.Fields(s), " ")
}

type animeTitlesDoc struct {
	Anime []animeTitleItem `xml:"anime"`
}

type animeTitleItem struct {
	AID    string           `xml:"aid,attr"`
	Titles []animeTitleName `xml:"title"`
}

type animeTitleName struct {
	Type  string `xml:"type,attr"`
	Value string `xml:",chardata"`
}

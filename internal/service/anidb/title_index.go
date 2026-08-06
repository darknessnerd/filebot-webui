package anidb

import (
	"encoding/xml"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var titleNormPattern = regexp.MustCompile(`[^a-z0-9]+`)

type titleIndex struct {
	byTitle map[string][]int
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

	idx := &titleIndex{byTitle: map[string][]int{}}
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
			idx.byTitle[key] = appendUniqueAID(idx.byTitle[key], aid)
		}
	}

	return idx, nil
}

func (i *titleIndex) FindAID(query string) (int, error) {
	if i == nil {
		return 0, fmt.Errorf("title index not configured")
	}
	key := normalizeLookupTitle(query)
	if key == "" {
		return 0, fmt.Errorf("empty lookup title")
	}
	aids := i.byTitle[key]
	if len(aids) == 0 {
		return 0, fmt.Errorf("no aid found for %q", query)
	}
	if len(aids) > 1 {
		return 0, fmt.Errorf("ambiguous title %q (multiple aids)", query)
	}
	return aids[0], nil
}

func normalizeLookupTitle(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = titleNormPattern.ReplaceAllString(s, " ")
	return strings.Join(strings.Fields(s), " ")
}

func appendUniqueAID(existing []int, aid int) []int {
	for _, v := range existing {
		if v == aid {
			return existing
		}
	}
	return append(existing, aid)
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

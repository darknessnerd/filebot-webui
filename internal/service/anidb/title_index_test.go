package anidb

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadTitleIndexFromFile_AndFindAID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "anime-titles.xml")
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<animetitles>
  <anime aid="1">
    <title xml:lang="x-jat" type="main">Seikai no Monshou</title>
    <title xml:lang="en" type="official">Crest of the Stars</title>
    <title xml:lang="en" type="short">CotS</title>
    <title xml:lang="ja" type="kana">せいかいのもんしょう</title>
  </anime>
  <anime aid="2">
    <title type="main">Dirty Pair Flash</title>
  </anime>
</animetitles>`
	require.NoError(t, os.WriteFile(path, []byte(xmlData), 0644))

	idx, err := loadTitleIndexFromFile(path)
	require.NoError(t, err)

	aid, err := idx.FindAID("Seikai no Monshou")
	require.NoError(t, err)
	assert.Equal(t, 1, aid)

	aid, err = idx.FindAID("Crest of the Stars")
	require.NoError(t, err)
	assert.Equal(t, 1, aid)

	aid, err = idx.FindAID("dirty-pair.flash")
	require.NoError(t, err)
	assert.Equal(t, 2, aid)

	// short and kana types must not be indexed
	_, err = idx.FindAID("CotS")
	require.Error(t, err)

	_, err = idx.FindAID("せいかいのもんしょう")
	require.Error(t, err)
}

func TestTitleIndex_AmbiguousTitle(t *testing.T) {
	idx := &titleIndex{
		byTitle: map[string][]int{
			"ghost in the shell": {1, 2},
		},
	}
	_, err := idx.FindAID("Ghost in the Shell")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ambiguous")
}

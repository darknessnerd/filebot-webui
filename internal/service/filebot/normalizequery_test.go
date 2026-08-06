package filebot

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeQuery_MovieTorrentNames(t *testing.T) {
	cases := []struct {
		input    string
		wantQ    string
		wantYear int
	}{
		// --- dot-separated (classic torrent filename) ---
		{
			input:    "Dune.Part.Two.2024.2160p.UHD.BluRay.mkv",
			wantQ:    "Dune Part Two",
			wantYear: 2024,
		},
		{
			input:    "The.Matrix.1999.1080p.BluRay.mkv",
			wantQ:    "The Matrix",
			wantYear: 1999,
		},
		// --- space-separated Italian forum style ---
		{
			input:    "Under the Silver Lake (2018) 1080p x265 ITA ENG AAC [WEBRip]",
			wantQ:    "Under the Silver Lake",
			wantYear: 2018,
		},
		{
			input:    "Emergency Declaration (2021) 4K H265 DoVi ITA AAC KOR DTS MULTISUB",
			wantQ:    "Emergency Declaration",
			wantYear: 2021,
		},
		{
			input:    "Il Divo (2008) [1080p x265 10b ITA AC3 MULTISUB Bluray] - GEGE",
			wantQ:    "Il Divo",
			wantYear: 2008,
		},
		{
			input:    "Doctor Strange nel Multiverso della Follia (2022) IMAX 1080p AV1 ITA ENG Opus 5.1 Sub Ita-MIRCrew",
			wantQ:    "Doctor Strange nel Multiverso della Follia",
			wantYear: 2022,
		},
		{
			input:    "Nouvelle Vague (2025) 1080p h265 FRE AAC Sub ITA ENG [gelderm]",
			wantQ:    "Nouvelle Vague",
			wantYear: 2025,
		},
		{
			input:    "Quién sabe? (1967) VERSIONE INTEGRALE [1080p x264 ITA DTS 2.0 Sub Ita] by phadron MIRCrew",
			wantQ:    "Quién sabe?",
			wantYear: 1967,
		},
		// --- Italian+English dual title ---
		{
			input:    "Mamma, ho perso l'aereo - Home Alone (1990) 1080p.H265 AC3 5.1 ITA.ENG sub ita.eng MIRCrew",
			wantQ:    "Mamma, ho perso l'aereo Home Alone",
			wantYear: 1990,
		},
		{
			input:    "Lucía y el sexo - Lucia e il sesso (2001) 1080p H264 AC3 5.1 ITA SPA ENG MULTISUB BDMux Unrated-jU1C3 [8.74 Gb]",
			wantQ:    "Lucía y el sexo Lucia e il sesso",
			wantYear: 2001,
		},
		{
			input:    "L'ultima missione: Project Hail Mary (2026) 1080p.H265 AC3 5.1 ITA.ENG sub ita.eng [Paso77]",
			wantQ:    "L'ultima missione: Project Hail Mary",
			wantYear: 2026,
		},
		// --- title starts with year-like number (regression: must not grab wrong year) ---
		{
			input:    "2001: A Space Odyssey (1968)",
			wantQ:    "2001: A Space Odyssey",
			wantYear: 1968,
		},
		// --- no year ---
		{
			input:    "S1m0ne (200) 1080p.H265 AC3 5.1 ITA.ENG sub ita.eng [Paso77]",
			wantQ:    "S1m0ne",
			wantYear: 0,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.input[:safeMin(40, len(tc.input))], func(t *testing.T) {
			gotQ, gotYear := normalizeQuery(tc.input, false)
			assert.Equal(t, tc.wantYear, gotYear, "year")
			assert.Equal(t, tc.wantQ, gotQ, "query")
		})
	}
}

func TestNormalizeQuery_TVTorrentNames(t *testing.T) {
	cases := []struct {
		input    string
		wantQ    string
		wantYear int
	}{
		// --- regression: episode title after SxxExx must be stripped ---
		// "Catfish Hunter" is episode 2 title; query must be "Futurama" only.
		{
			input:    "Futurama.S14E02.Catfish.Hunter.1080p.DSNP.WEB-DL.ENG.ITA.DDP5.1.H264-TheBlackKing.mkv",
			wantQ:    "Futurama",
			wantYear: 0,
		},
		// --- regression: season-year in filename ≠ show premiere year ---
		// Year 2026 is the current season, not Rick and Morty's premiere (2013).
		// normalizeQuery should still extract 2026 so SearchTV can attempt+retry.
		{
			input:    "Rick and Morty - Stagione 09 (2026).mkv",
			wantQ:    "Rick and Morty",
			wantYear: 2026,
		},
		// --- standard SxxExx ---
		{
			input:    "Breaking.Bad.S01E01.1080p.BluRay.mkv",
			wantQ:    "Breaking Bad",
			wantYear: 0,
		},
		{
			input:    "Breaking.Bad.S01E01E02.1080p.mkv",
			wantQ:    "Breaking Bad",
			wantYear: 0,
		},
		{
			input:    "The.Sopranos.S01E01.1080p.BluRay.ITA.ENG.mkv",
			wantQ:    "The Sopranos",
			wantYear: 0,
		},
		// --- Italian "Stagione N (YEAR)" format ---
		{
			input:    "Affari a quattro ruote - Stagione 1 (2003) [COMPLETA] 1080p H264 ITA AAC",
			wantQ:    "Affari a quattro ruote",
			wantYear: 2003,
		},
		{
			input:    "LOL - Chi ride è fuori - Stagione 6 (2026) [COMPLETA] 1080p H264 ITA EAC3 5.1 SUB ITA ENG",
			wantQ:    "LOL Chi ride è fuori",
			wantYear: 2026,
		},
		{
			input:    "Barbascura X - Sono Qui per Caos (2026) [COMPLETA] 1080p H264 ITA EAC3 MULTISUB",
			wantQ:    "Barbascura X Sono Qui per Caos",
			wantYear: 2026,
		},
		// --- year range in parens (YYYY-YYYY) ---
		{
			input:    "MasterChef Italia - Stagione 12 (2022-2023) [COMPLETA] SD H264 ITA AAC",
			wantQ:    "MasterChef Italia",
			wantYear: 2022,
		},
		{
			input:    "Diario di una nerd superstar - Stagioni 1-5 (2011-2016) [COMPLETA] - 1080p H264 ITA AAC WEB-DL ESTA",
			wantQ:    "Diario di una nerd superstar",
			wantYear: 2011,
		},
		// --- abbreviated year range (YYYY-YY) ---
		{
			input:    "FBI - Stagione 7 (2024-25) [IN CORSO 11/17] 1080p H264 ITA AAC3 2.0 ENG EAC3 5.1 SUB ITA ENG [kovalski]",
			wantQ:    "FBI",
			wantYear: 2024,
		},
		// --- standalone S-only season (no episode number) ---
		{
			input:    "Elsbeth - S03 (2025-2026) [IN CORSO] [13 / 20] - 1080p H264 ITA ENG AAC SUB ITA-ENG WEB-DL ESTA",
			wantQ:    "Elsbeth",
			wantYear: 2025,
		},
		{
			input:    "Drag Race Italia - After the race S01 (2021) [COMPLETA] 1080p H264 Ita Aac - by Carluz",
			wantQ:    "Drag Race Italia After the race",
			wantYear: 2021,
		},
		// --- Italian "Edizione N" ---
		{
			input:    "Presa Diretta - Edizione 28 (2023) [COMPLETA] 1080p H264 Ita Aac",
			wantQ:    "Presa Diretta",
			wantYear: 2023,
		},
		// --- paren subtitle in title (stripped, not kept) + standalone S-only ---
		{
			input:    "Il meglio (o quasi) di the Grand Tour - S01 (COMPLETA) 1080p H265 EAC3 5.1 ITA sub ITA WEBDL",
			wantQ:    "Il meglio di the Grand Tour",
			wantYear: 0,
		},
		// --- no year, long title with brackets; release group "MIRCrew" outside brackets is kept ---
		{
			input:    "Star Trek: Strange New Worlds - Stagione 4 [IN CORSO][01-02/10] 1080p h265 10bit Ita Eng Spa Ac3 Sub Ita Eng Spa MIRCrew",
			wantQ:    "Star Trek: Strange New Worlds MIRCrew",
			wantYear: 0,
		},
		// --- air date in parens "(DD/MM/YYYY)" — orphan paren cleanup ---
		{
			input:    "Report Rai - Smart tv is watching you (27/12/2021) 1080p.H264.Ita.Aac",
			wantQ:    "Report Rai Smart tv is watching you",
			wantYear: 2021,
		},
		// --- paren with descriptive text containing year "(Fuori orario, YYYY)" ---
		{
			input:    "Conversazione con Ryusuke Hamaguchi (Fuori orario, 2023) 720p H264 AAC ITA hardsub ITA",
			wantQ:    "Conversazione con Ryusuke Hamaguchi",
			wantYear: 2023,
		},
		// --- "+" separator in title ---
		{
			input:    "Nuova Scena - Rhythm + Flow Italia - Stagione 3 (2026) [COMPLETA] 1080p x264 ITA ENG EAC3 5.1 MULTISUB",
			wantQ:    "Nuova Scena Rhythm Flow Italia",
			wantYear: 2026,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.input[:safeMin(40, len(tc.input))], func(t *testing.T) {
			gotQ, gotYear := normalizeQuery(tc.input, true)
			assert.Equal(t, tc.wantYear, gotYear, "year")
			assert.Equal(t, tc.wantQ, gotQ, "query")
		})
	}
}

func TestNormalizeQuery_AnimeCorpusNames(t *testing.T) {
	cases := []struct {
		input    string
		wantQ    string
		wantYear int
	}{
		{
			input:    "A.D. Police: To Protect and Serve (1999) [COMPLETA] [SD H265 FLAC ENG JPN SUB ENG ITA]",
			wantQ:    "A D Police: To Protect and Serve",
			wantYear: 1999,
		},
		{
			input:    "Fate/Stay Night Unlimited Blade Works (2014) [1080p.h265.Jap.TrueHD.Ita.AC3.Sub.Jap.Eng]",
			wantQ:    "Fate/Stay Night Unlimited Blade Works",
			wantYear: 2014,
		},
		{
			input:    "Soul Land / Douluo Dalu (2018-2023) [COMPLETA] [1080p H264 AC3 CHI SUB ENG ITA]",
			wantQ:    "Soul Land / Douluo Dalu",
			wantYear: 2018,
		},
		{
			input:    "Il fichissimo del baseball (1977) [STAGIONE UNICA] [COMPLETA] [1080p H265 ITA AC3 JAP EAC3]",
			wantQ:    "Il fichissimo del baseball",
			wantYear: 1977,
		},
		{
			input:    "Bartender: Glass of God - Bartender: Kami no Glass (2024) [COMPLETA] [12/12] [1080p H264 JAP VORBIS 2.0 SUB ITA]",
			wantQ:    "Bartender: Glass of God Bartender: Kami no Glass",
			wantYear: 2024,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.input[:safeMin(40, len(tc.input))], func(t *testing.T) {
			gotQ, gotYear := normalizeQuery(tc.input, true)
			assert.Equal(t, tc.wantYear, gotYear, "year")
			assert.Equal(t, tc.wantQ, gotQ, "query")
		})
	}
}

func safeMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

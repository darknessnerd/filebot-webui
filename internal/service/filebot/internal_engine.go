package filebot

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"github.com/darknessnerd/filebot-webui/internal/domain"
	"github.com/darknessnerd/filebot-webui/internal/logger"
)

var (
	yearPattern = regexp.MustCompile(`\b(19|20)\d{2}\b`)
	// Matches (YYYY), (YYYY-YYYY), or (YYYY-YY) year ranges.
	// Must be applied BEFORE sep normalization while the dash is still intact.
	yearInParenPattern       = regexp.MustCompile(`\((19|20)\d{2}(?:[-/]\d{2,4})?\)`)
	episodePattern           = regexp.MustCompile(`(?i)(?:s(\d{1,2})[.\s]?e(\d{1,2})(?:[-]?e(\d{1,2}))?|(\d{1,2})x(\d{1,2}))`)
	animeEpisodePattern      = regexp.MustCompile(`(?i)\b(?:e|ep)\s*0*(\d{1,3})(?:\s*[-_]\s*0*(\d{1,3}))?\b`)
	// "- 01", "- 001" bare episode after dash separator (SubsPlease/Erai-raws style).
	animeBareEpPattern = regexp.MustCompile(`(?:^|[\s._-])-\s*0*(\d{1,3})(?:\s*-\s*0*(\d{1,3}))?(?:\s|$|\.)`)
	// "#01" / "#001"
	animeHashEpPattern = regexp.MustCompile(`#0*(\d{1,3})(?:\s*-\s*0*(\d{1,3}))?`)
	// "Part 1" / "Part I" (OVA style)
	animePartPattern = regexp.MustCompile(`(?i)\bpart\s+([IVXLC]+|\d{1,2})\b`)
	// "OVA 1" / "OVA1" / "SP 1" / "SP1" / "Special 1"
	animeSpecialEpPattern = regexp.MustCompile(`(?i)\b(?:ova|sp|special)\s*0*(\d{1,2})\b`)
	animeProgressPattern  = regexp.MustCompile(`\[(\d{1,3})(?:\s*-\s*(\d{1,3}))?\s*(?:/|-)\s*(\d{1,3}|XX)\]`)
	animeSeasonPattern    = regexp.MustCompile(`(?i)\b(?:stagione|season)\s*(\d{1,2})\b`)
	animeSingleSeasonPattern = regexp.MustCompile(`(?i)\bstagione\s+unica\b`)
	// Italian "Stagione N" or "Stagioni N M" (after sep, range becomes space-separated digits).
	stagionPattern = regexp.MustCompile(`(?i)\bstagion[ie](?:\s+\d+)*\b`)
	// Italian "Edizione N".
	edizionPattern = regexp.MustCompile(`(?i)\bedizione(?:\s+\d+)*\b`)
	// Standalone season marker without episode, e.g. "S03" (without E).
	seasonOnlyPattern = regexp.MustCompile(`(?i)\bs\d{1,2}\b`)
	// Unclosed paren group at end of string, left after year truncation
	// e.g. "(Fuori orario," or "(27/12/" from air-date parens.
	orphanParenPattern = regexp.MustCompile(`\([^)]*$`)
	noisePattern       = regexp.MustCompile(`(?i)\b(?:1080p|720p|2160p|480p|4k|sd|imax|dovi|hdr10|hdr|bluray|blu[\s.-]?ray|webrip|web[\s.-]?dl|brrip|hdrip|bdmux|bdrip|bdremux|remux|dvdrip|dvd|hdtv|repack|proper|extended|uncut|unrated|fanedit|versione integrale|x264|x265|h264|h265|h262|hevc|av1|aac|ac3|eac3|e[\s-]?ac3|dts|dolby|opus|flac|pcm|multisub|sub|nuita|nueng|sample|miniserie|hardsub|multilang|10bit)\b`)
	langPattern        = regexp.MustCompile(`(?i)\b(?:ita|eng|spa|fre|ger|rus|jpn|kor|por|ara|fil|dut|swe|dan|nor|fin|tur|hin|slo|cze|pol|hun)\b`)
	// Must run BEFORE filepath.Ext and before sep: "5.1" in a non-file string would otherwise be
	// detected as the file extension ".1 …" by filepath.Ext.
	audioChannelPattern = regexp.MustCompile(`\b\d+\.\d+\b`)
	bracketPattern      = regexp.MustCompile(`\[[^\]]*\]`)
	parenPattern        = regexp.MustCompile(`\([^)]*\)`)
	// Include "+" so "Rhythm + Flow" becomes "Rhythm Flow" rather than "Rhythm + Flow".
	sepPattern = regexp.MustCompile(`[._+\-]+`)
)

var videoExtensions = map[string]bool{
	".mkv": true, ".mp4": true, ".avi": true, ".m4v": true, ".mov": true, ".wmv": true,
}

var subtitleExtensions = map[string]bool{
	".srt": true, ".ass": true, ".sub": true, ".vtt": true, ".ssa": true,
}

type movieResolver interface {
	SearchMovie(ctx context.Context, query string, year int) (*domain.MovieMatch, error)
}

type tvResolver interface {
	SearchTV(ctx context.Context, query string, year int) (*domain.TVMatch, error)
}

type metadataResolver interface {
	movieResolver
	tvResolver
	animeResolver
}

type InternalEngine struct {
	resolver metadataResolver
	log      logger.Logger
}

func NewInternal(mediaRoot string, resolver metadataResolver, log logger.Logger) *Service {
	return New(mediaRoot, NewInternalEngine(resolver, log), log)
}

func NewInternalEngine(resolver metadataResolver, log logger.Logger) *InternalEngine {
	return &InternalEngine{resolver: resolver, log: log}
}

func (e *InternalEngine) Execute(ctx context.Context, job domain.FileBotJob) (domain.FileBotResult, error) {
	if job.Filter != "" {
		return domain.FileBotResult{Errors: []string{"native engine does not support --filter yet"}}, fmt.Errorf("%w: native engine does not support --filter yet", domain.ErrInvalidArg)
	}
	if job.Format != "" && job.Format != "{plex}" {
		return domain.FileBotResult{Errors: []string{"native engine supports default format only"}}, fmt.Errorf("%w: native engine supports default format only", domain.ErrInvalidArg)
	}

	e.log.Debug().
		Strs("source_paths", job.SourcePaths).
		Str("db", job.DB).
		Str("action", job.Action).
		Str("conflict", job.Conflict).
		Str("output", job.Output).
		Bool("recursive", job.Recursive).
		Bool("dry_run", job.Action == "test").
		Msg("internal engine: executing job")

	files, err := collectVideoFiles(job.SourcePaths, job.Recursive, job.Action == "test")
	if err != nil {
		return domain.FileBotResult{Errors: []string{err.Error()}}, fmt.Errorf("%w: %v", domain.ErrFileBotFailed, err)
	}
	if len(files) == 0 {
		err := fmt.Errorf("no video files found in selected source paths")
		return domain.FileBotResult{Errors: []string{err.Error()}}, fmt.Errorf("%w: %v", domain.ErrFileBotFailed, err)
	}

	e.log.Debug().Strs("files", files).Int("count", len(files)).Msg("internal engine: collected video files")

	switch job.DB {
	case "TheMovieDB":
		return e.executeMovies(ctx, job, files)
	case "TheMovieDB::TV":
		return e.executeTV(ctx, job, files)
	case "AniDB":
		return e.executeAnime(ctx, job, files)
	default:
		return domain.FileBotResult{Errors: []string{"native engine supports TheMovieDB, TheMovieDB::TV, and AniDB only"}}, fmt.Errorf("%w: native engine supports TheMovieDB, TheMovieDB::TV, and AniDB only", domain.ErrInvalidArg)
	}
}

func (e *InternalEngine) executeMovies(ctx context.Context, job domain.FileBotJob, files []string) (domain.FileBotResult, error) {
	cache := map[string]*domain.MovieMatch{}
	var result domain.FileBotResult

	for _, source := range files {
		q, y := deriveQueryForFile(job.Query, source, false)
		key := fmt.Sprintf("%s|%d", q, y)

		match, ok := cache[key]
		if !ok {
			e.log.Debug().Str("query", q).Int("year", y).Msg("internal engine: movie query derived")
			var err error
			match, err = e.resolver.SearchMovie(ctx, q, y)
			if err != nil {
				appendResult(&result, "", fmt.Errorf("no match for %s: %w", filepath.Base(source), err))
				continue
			}
			e.log.Debug().Str("title", match.Title).Int("year", match.Year).Int("tmdb_id", match.ID).Msg("internal engine: movie match found")
			cache[key] = match
		}

		title := sanitizeName(match.Title)
		folderName := title
		if match.Year > 0 {
			folderName = fmt.Sprintf("%s (%d)", title, match.Year)
		}

		target := filepath.Join(job.Output, "Movies", folderName, title+filepath.Ext(source))
		e.log.Debug().Str("source", source).Str("target", target).Str("action", job.Action).Msg("internal engine: applying action")
		msg, err := applyAction(job.Action, job.Conflict, source, target)
		appendResult(&result, msg, err)

		if job.Action != "test" {
			for _, sub := range findCompanionSubtitles(source, e.log) {
				subTarget := subtitleTargetName(target, source, sub)
				e.log.Debug().Str("sub", sub).Str("target", subTarget).Msg("internal engine: moving subtitle")
				msg, err := applyAction(job.Action, job.Conflict, sub, subTarget)
				appendResult(&result, msg, err)
			}
		}
	}

	result.RawOutput = strings.Join(append(result.Successes, result.Errors...), "\n")
	if len(result.Errors) > 0 {
		return result, fmt.Errorf("%w: movie operations failed", domain.ErrFileBotFailed)
	}
	return result, nil
}

func (e *InternalEngine) executeTV(ctx context.Context, job domain.FileBotJob, files []string) (domain.FileBotResult, error) {
	cache := map[string]*domain.TVMatch{}
	var result domain.FileBotResult

	for _, source := range files {
		q, y := deriveQueryForFile(job.Query, source, true)
		key := fmt.Sprintf("%s|%d", q, y)

		match, ok := cache[key]
		if !ok {
			e.log.Debug().Str("query", q).Int("year", y).Msg("internal engine: tv query derived")
			var err error
			match, err = e.resolver.SearchTV(ctx, q, y)
			if err != nil {
				appendResult(&result, "", fmt.Errorf("no match for %s: %w", filepath.Base(source), err))
				continue
			}
			e.log.Debug().Str("name", match.Name).Int("year", match.Year).Int("tmdb_id", match.ID).Msg("internal engine: tv match found")
			cache[key] = match
		}

		season, firstEp, lastEp, err := extractEpisode(source)
		if err != nil {
			appendResult(&result, "", err)
			continue
		}

		showName := sanitizeName(match.Name)
		epLabel := fmt.Sprintf("S%02dE%02d", season, firstEp)
		if lastEp > 0 && lastEp != firstEp {
			epLabel = fmt.Sprintf("S%02dE%02d-E%02d", season, firstEp, lastEp)
		}

		target := filepath.Join(
			job.Output, "TV", showName,
			fmt.Sprintf("Season %d", season),
			fmt.Sprintf("%s - %s%s", showName, epLabel, filepath.Ext(source)),
		)
		e.log.Debug().Str("source", source).Str("target", target).Int("season", season).Int("first_ep", firstEp).Int("last_ep", lastEp).Str("action", job.Action).Msg("internal engine: applying action")
		msg, err := applyAction(job.Action, job.Conflict, source, target)
		appendResult(&result, msg, err)

		if job.Action != "test" {
			for _, sub := range findCompanionSubtitles(source, e.log) {
				subTarget := subtitleTargetName(target, source, sub)
				e.log.Debug().Str("sub", sub).Str("target", subTarget).Msg("internal engine: moving subtitle")
				msg, err := applyAction(job.Action, job.Conflict, sub, subTarget)
				appendResult(&result, msg, err)
			}
		}
	}

	result.RawOutput = strings.Join(append(result.Successes, result.Errors...), "\n")
	if len(result.Errors) > 0 {
		return result, fmt.Errorf("%w: tv operations failed", domain.ErrFileBotFailed)
	}
	return result, nil
}

func (e *InternalEngine) executeAnime(ctx context.Context, job domain.FileBotJob, files []string) (domain.FileBotResult, error) {
	var (
		match *domain.AnimeMatch
		err   error
	)
	aid, aidErr := parseAniDBAID(job.Query)
	if aidErr == nil {
		match, err = e.resolver.SearchAnimeByAID(ctx, aid)
		if err != nil {
			return domain.FileBotResult{Errors: []string{fmt.Sprintf("AniDB lookup failed for aid=%d: %v", aid, err)}}, fmt.Errorf("%w: anidb lookup failed: %v", domain.ErrFileBotFailed, err)
		}
		e.log.Debug().Int("aid", aid).Str("title", match.Title).Int("year", match.Year).Int("anidb_id", match.ID).Msg("internal engine: anime match found by aid")
	} else {
		query, year := deriveQueryForFile(job.Query, files[0], true)
		match, err = e.resolver.SearchAnime(ctx, query, year)
		if err != nil {
			errMsg := fmt.Sprintf("AniDB title lookup failed for %q: %v", query, err)
			lowerErr := strings.ToLower(err.Error())
			if strings.Contains(lowerErr, "ambiguous") || strings.Contains(lowerErr, "no aid found") {
				errMsg += ". Try --q aid:<id> (example: aid:1)"
			}
			return domain.FileBotResult{Errors: []string{errMsg}}, fmt.Errorf("%w: anidb title lookup failed: %v", domain.ErrFileBotFailed, err)
		}
		e.log.Debug().Str("query", query).Int("year", year).Str("title", match.Title).Int("anidb_id", match.ID).Msg("internal engine: anime match found by title")
	}

	var result domain.FileBotResult

	for _, source := range files {

		season, firstEp, lastEp, err := extractAnimeEpisode(source)
		if err != nil {
			appendResult(&result, "", err)
			continue
		}

		animeName := sanitizeName(match.Title)
		epLabel := fmt.Sprintf("S%02dE%02d", season, firstEp)
		if lastEp > 0 && lastEp != firstEp {
			epLabel = fmt.Sprintf("S%02dE%02d-E%02d", season, firstEp, lastEp)
		}

		target := filepath.Join(
			job.Output, "Anime", animeName,
			fmt.Sprintf("Season %d", season),
			fmt.Sprintf("%s - %s%s", animeName, epLabel, filepath.Ext(source)),
		)
		e.log.Debug().Str("source", source).Str("target", target).Int("season", season).Int("first_ep", firstEp).Int("last_ep", lastEp).Str("action", job.Action).Msg("internal engine: applying anime action")
		msg, err := applyAction(job.Action, job.Conflict, source, target)
		appendResult(&result, msg, err)

		if job.Action != "test" {
			for _, sub := range findCompanionSubtitles(source, e.log) {
				subTarget := subtitleTargetName(target, source, sub)
				e.log.Debug().Str("sub", sub).Str("target", subTarget).Msg("internal engine: moving anime subtitle")
				msg, err := applyAction(job.Action, job.Conflict, sub, subTarget)
				appendResult(&result, msg, err)
			}
		}
	}

	result.RawOutput = strings.Join(append(result.Successes, result.Errors...), "\n")
	if len(result.Errors) > 0 {
		return result, fmt.Errorf("%w: anime operations failed", domain.ErrFileBotFailed)
	}
	return result, nil
}

func parseAniDBAID(query string) (int, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return 0, fmt.Errorf("missing aid")
	}
	if strings.HasPrefix(strings.ToLower(q), "aid:") {
		q = strings.TrimSpace(q[4:])
	}
	if strings.HasPrefix(strings.ToLower(q), "aid=") {
		q = strings.TrimSpace(q[4:])
	}
	aid, err := strconv.Atoi(q)
	if err != nil || aid <= 0 {
		return 0, fmt.Errorf("invalid aid")
	}
	return aid, nil
}

func collectVideoFiles(sourcePaths []string, recursive bool, dryRun bool) ([]string, error) {
	var files []string
	for _, source := range sourcePaths {
		if dryRun {
			// test action: treat each source as a virtual path, no filesystem access
			if isVideoFile(source) {
				files = append(files, source)
			} else {
				// source is likely a directory name without extension — include as-is with fake ext
				files = append(files, source+".mkv")
			}
			continue
		}
		info, err := os.Stat(source)
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", source, err)
		}
		if !info.IsDir() {
			if isVideoFile(source) {
				files = append(files, source)
			}
			continue
		}

		err = filepath.WalkDir(source, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				if path != source && !recursive {
					return filepath.SkipDir
				}
				return nil
			}
			if isVideoFile(path) {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk %s: %w", source, err)
		}
	}
	return files, nil
}

func deriveQueryForFile(jobQuery, fileFallback string, isTV bool) (string, int) {
	if jobQuery != "" {
		return normalizeQuery(jobQuery, isTV)
	}
	return normalizeQuery(filepath.Base(fileFallback), isTV)
}

func normalizeQuery(raw string, stripEpisode bool) (string, int) {
	// Strip audio channel specs FIRST — before filepath.Ext, which would otherwise
	// treat e.g. "5.1 ITA sub" as extension ".1 ITA sub" on strings without a real ext.
	raw = audioChannelPattern.ReplaceAllString(raw, " ")
	ext := strings.ToLower(filepath.Ext(raw))
	if videoExtensions[ext] || subtitleExtensions[ext] {
		raw = strings.TrimSuffix(raw, filepath.Ext(raw))
	}

	// Prefer year in parentheses "(YYYY)" or range "(YYYY-YYYY)"/"(YYYY-YY)".
	// Must run BEFORE sep normalization while the dash in a year range is still intact.
	// Prevents titles that start with a year-like number (e.g. "2001: A Space Odyssey") from
	// being mis-detected by the bare-year fallback.
	year := 0
	if loc := yearInParenPattern.FindStringIndex(raw); loc != nil {
		// Always take the first 4 digits right after the opening paren (handles ranges too).
		yearText := raw[loc[0]+1 : loc[0]+5]
		if y, err := strconv.Atoi(yearText); err == nil {
			year = y
		}
		raw = strings.TrimRight(raw[:loc[0]], " ")
	}

	raw = sepPattern.ReplaceAllString(raw, " ")

	// Fallback: bare year on sep-normalised string.
	if year == 0 {
		if loc := yearPattern.FindStringIndex(raw); loc != nil {
			yearText := raw[loc[0]:loc[1]]
			if y, err := strconv.Atoi(yearText); err == nil {
				year = y
			}
			raw = strings.TrimRight(raw[:loc[0]], " ([{")
		}
	}

	if stripEpisode {
		raw = episodePattern.ReplaceAllString(raw, " ")
		raw = animeEpisodePattern.ReplaceAllString(raw, " ")
		raw = animeBareEpPattern.ReplaceAllString(raw, " ")
		raw = animeHashEpPattern.ReplaceAllString(raw, " ")
		raw = animePartPattern.ReplaceAllString(raw, " ")
		raw = animeSpecialEpPattern.ReplaceAllString(raw, " ")
		// Italian season / edition markers — "Stagione N", "Stagioni N M", "Edizione N".
		raw = stagionPattern.ReplaceAllString(raw, " ")
		raw = edizionPattern.ReplaceAllString(raw, " ")
		// Standalone season marker without episode number, e.g. "S03".
		raw = seasonOnlyPattern.ReplaceAllString(raw, " ")
	}

	// Strip bracket/paren groups: release tags, alternate titles, etc.
	// orphanParenPattern cleans up unclosed groups left after year truncation,
	// e.g. "(Fuori orario," or "(27/12/" from air-date parens.
	raw = bracketPattern.ReplaceAllString(raw, " ")
	raw = parenPattern.ReplaceAllString(raw, " ")
	raw = orphanParenPattern.ReplaceAllString(raw, " ")
	// Strip language codes and technical noise tokens.
	raw = langPattern.ReplaceAllString(raw, " ")
	raw = noisePattern.ReplaceAllString(raw, " ")

	raw = strings.Join(strings.Fields(raw), " ")
	return strings.TrimSpace(raw), year
}

func extractEpisode(path string) (season, firstEp, lastEp int, err error) {
	base := filepath.Base(path)
	matches := episodePattern.FindStringSubmatch(base)
	if matches == nil {
		return 0, 0, 0, fmt.Errorf("no episode marker found in %s", base)
	}

	// Groups: s(\d{1,2})e(\d{1,2})(?:[-]?e(\d{1,2}))?  OR  (\d{1,2})x(\d{1,2})
	//         1          2           3                        4          5
	var seasonStr, firstStr, lastStr string
	if matches[1] != "" {
		seasonStr, firstStr, lastStr = matches[1], matches[2], matches[3]
	} else {
		seasonStr, firstStr = matches[4], matches[5]
	}

	season, err = strconv.Atoi(seasonStr)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid season in %s", base)
	}
	firstEp, err = strconv.Atoi(firstStr)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid episode in %s", base)
	}
	if lastStr != "" {
		lastEp, err = strconv.Atoi(lastStr)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("invalid last episode in %s", base)
		}
	}
	return season, firstEp, lastEp, nil
}

func extractAnimeEpisode(path string) (season, firstEp, lastEp int, err error) {
	if season, firstEp, lastEp, err = extractEpisode(path); err == nil {
		return season, firstEp, lastEp, nil
	}

	season = deriveAnimeSeason(path)
	base := filepath.Base(path)
	if matches := animeEpisodePattern.FindStringSubmatch(base); matches != nil {
		firstEp, err = strconv.Atoi(matches[1])
		if err != nil {
			return 0, 0, 0, fmt.Errorf("invalid anime episode in %s", base)
		}
		if matches[2] != "" {
			lastEp, err = strconv.Atoi(matches[2])
			if err != nil {
				return 0, 0, 0, fmt.Errorf("invalid anime last episode in %s", base)
			}
		}
		return season, firstEp, lastEp, nil
	}

	if matches := animeProgressPattern.FindStringSubmatch(path); matches != nil {
		firstEp, err = strconv.Atoi(matches[1])
		if err != nil {
			return 0, 0, 0, fmt.Errorf("invalid anime progress episode in %s", base)
		}
		if matches[2] != "" {
			lastEp, err = strconv.Atoi(matches[2])
			if err != nil {
				return 0, 0, 0, fmt.Errorf("invalid anime progress last episode in %s", base)
			}
		}
		return season, firstEp, lastEp, nil
	}

	if matches := animeBareEpPattern.FindStringSubmatch(base); matches != nil {
		firstEp, err = strconv.Atoi(matches[1])
		if err != nil {
			return 0, 0, 0, fmt.Errorf("invalid bare episode in %s", base)
		}
		if matches[2] != "" {
			lastEp, err = strconv.Atoi(matches[2])
			if err != nil {
				return 0, 0, 0, fmt.Errorf("invalid bare episode range in %s", base)
			}
		}
		return season, firstEp, lastEp, nil
	}

	if matches := animeHashEpPattern.FindStringSubmatch(base); matches != nil {
		firstEp, err = strconv.Atoi(matches[1])
		if err != nil {
			return 0, 0, 0, fmt.Errorf("invalid hash episode in %s", base)
		}
		if matches[2] != "" {
			lastEp, err = strconv.Atoi(matches[2])
			if err != nil {
				return 0, 0, 0, fmt.Errorf("invalid hash episode range in %s", base)
			}
		}
		return season, firstEp, lastEp, nil
	}

	if matches := animeSpecialEpPattern.FindStringSubmatch(base); matches != nil {
		firstEp, err = strconv.Atoi(matches[1])
		if err != nil {
			return 0, 0, 0, fmt.Errorf("invalid special episode in %s", base)
		}
		return season, firstEp, 0, nil
	}

	if matches := animePartPattern.FindStringSubmatch(base); matches != nil {
		firstEp = romanToInt(matches[1])
		if firstEp <= 0 {
			return 0, 0, 0, fmt.Errorf("invalid part number in %s", base)
		}
		return season, firstEp, 0, nil
	}

	return 0, 0, 0, fmt.Errorf("no anime episode marker found in %s", base)
}

func romanToInt(s string) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	vals := map[byte]int{'I': 1, 'V': 5, 'X': 10, 'L': 50, 'C': 100}
	s = strings.ToUpper(s)
	total := 0
	for i := 0; i < len(s); i++ {
		cur, ok := vals[s[i]]
		if !ok {
			return 0
		}
		if i+1 < len(s) {
			if next, ok2 := vals[s[i+1]]; ok2 && next > cur {
				total -= cur
				continue
			}
		}
		total += cur
	}
	return total
}

func deriveAnimeSeason(path string) int {
	if animeSingleSeasonPattern.MatchString(path) {
		return 1
	}
	matches := animeSeasonPattern.FindStringSubmatch(path)
	if len(matches) < 2 || matches[1] == "" {
		return 1
	}
	season, err := strconv.Atoi(matches[1])
	if err != nil || season <= 0 {
		return 1
	}
	return season
}

// findCompanionSubtitles returns subtitle files in the same directory whose
// stem matches or extends the video file stem (e.g. "movie.en.srt").
func findCompanionSubtitles(videoPath string, log logger.Logger) []string {
	dir := filepath.Dir(videoPath)
	stem := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))

	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Warn().Err(err).Str("dir", dir).Msg("internal engine: cannot read dir for companion subtitles")
		return nil
	}

	var subs []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		origExt := filepath.Ext(name)
		if !subtitleExtensions[strings.ToLower(origExt)] {
			continue
		}
		subStem := strings.TrimSuffix(name, origExt)
		if subStem == stem || strings.HasPrefix(subStem, stem+".") {
			subs = append(subs, filepath.Join(dir, name))
		}
	}
	return subs
}

// subtitleTargetName computes the destination path for a subtitle companion,
// preserving any language/flag suffix (e.g. ".en", ".forced").
func subtitleTargetName(videoTarget, videoSource, subSource string) string {
	videoSourceStem := strings.TrimSuffix(filepath.Base(videoSource), filepath.Ext(videoSource))
	subName := filepath.Base(subSource)
	subExt := filepath.Ext(subName)
	subStem := strings.TrimSuffix(subName, subExt)

	extra := ""
	if strings.HasPrefix(subStem, videoSourceStem) {
		extra = strings.TrimPrefix(subStem, videoSourceStem) // e.g. ".en"
	}
	videoTargetStem := strings.TrimSuffix(filepath.Base(videoTarget), filepath.Ext(videoTarget))
	return filepath.Join(filepath.Dir(videoTarget), videoTargetStem+extra+subExt)
}

func applyAction(action, conflict, source, target string) (string, error) {
	finalTarget, skipped, err := resolveTarget(target, conflict)
	if err != nil {
		return "", err
	}
	if skipped {
		return fmt.Sprintf("SKIP %s -> %s", source, finalTarget), nil
	}
	if action == "test" {
		return fmt.Sprintf("TEST %s -> %s", source, finalTarget), nil
	}

	if err := os.MkdirAll(filepath.Dir(finalTarget), 0755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", filepath.Dir(finalTarget), err)
	}

	switch action {
	case "move":
		if err := moveFile(source, finalTarget); err != nil {
			return "", err
		}
	case "copy":
		if err := copyFile(source, finalTarget); err != nil {
			return "", err
		}
	case "symlink":
		if err := os.Symlink(source, finalTarget); err != nil {
			return "", fmt.Errorf("symlink %s -> %s: %w", source, finalTarget, err)
		}
	case "hardlink":
		if err := os.Link(source, finalTarget); err != nil {
			return "", fmt.Errorf("hardlink %s -> %s: %w", source, finalTarget, err)
		}
	default:
		return "", fmt.Errorf("unsupported action %q", action)
	}
	return fmt.Sprintf("%s %s -> %s", strings.ToUpper(action), source, finalTarget), nil
}

func resolveTarget(target, conflict string) (string, bool, error) {
	_, err := os.Stat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return target, false, nil
		}
		return "", false, err
	}

	switch conflict {
	case "skip":
		return target, true, nil
	case "replace":
		if err := os.Remove(target); err != nil {
			return "", false, fmt.Errorf("remove existing %s: %w", target, err)
		}
		return target, false, nil
	case "index", "auto":
		indexed, err := nextIndexedPath(target)
		if err != nil {
			return "", false, err
		}
		return indexed, false, nil
	case "fail":
		return "", false, fmt.Errorf("target exists: %s", target)
	default:
		return "", false, fmt.Errorf("unsupported conflict mode %q", conflict)
	}
}

func nextIndexedPath(target string) (string, error) {
	dir := filepath.Dir(target)
	ext := filepath.Ext(target)
	base := strings.TrimSuffix(filepath.Base(target), ext)
	for i := 2; i <= 100; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s (%d)%s", base, i, ext))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not find unused index for %s after 100 attempts", filepath.Base(target))
}

func moveFile(source, target string) error {
	err := os.Rename(source, target)
	if err == nil {
		return nil
	}
	// Only fall back to copy+delete for cross-device moves; all other errors propagate.
	var linkErr *os.LinkError
	if !errors.As(err, &linkErr) || !errors.Is(linkErr.Err, syscall.EXDEV) {
		return fmt.Errorf("rename %s -> %s: %w", source, target, err)
	}
	if err := copyFile(source, target); err != nil {
		return err
	}
	if err := os.Remove(source); err != nil {
		return fmt.Errorf("remove source %s after cross-device move: %w", source, err)
	}
	return nil
}

func copyFile(source, target string) error {
	from, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open source %s: %w", source, err)
	}
	defer from.Close()

	// Write to a temp file in the target directory, then rename atomically.
	// This prevents a partially-written target if the process is interrupted.
	tmp, err := os.CreateTemp(filepath.Dir(target), ".tmp-copy-*")
	if err != nil {
		return fmt.Errorf("create temp for %s: %w", target, err)
	}
	tmpName := tmp.Name()

	if _, err := io.Copy(tmp, from); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("copy %s -> tmp: %w", source, err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp for %s: %w", target, err)
	}
	if err := os.Rename(tmpName, target); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename temp to %s: %w", target, err)
	}
	return nil
}

func isVideoFile(path string) bool {
	return videoExtensions[strings.ToLower(filepath.Ext(path))]
}

func sanitizeName(s string) string {
	replacer := strings.NewReplacer(":", " ", "/", " ", "\\", " ")
	s = replacer.Replace(s)
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimSpace(s)
}

func appendResult(result *domain.FileBotResult, success string, err error) {
	if success != "" {
		result.Successes = append(result.Successes, success)
	}
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
	}
}

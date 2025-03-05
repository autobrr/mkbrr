package seasonpack

import (
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/autobrr/mkbrr/internal/display"
	"github.com/autobrr/mkbrr/internal/types"
)

var minIntFunc func(a, b int) int

func Init(minIntFn func(a, b int) int) {
	minIntFunc = minIntFn
}

var seasonPackPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\.S(\d{1,2})\.(?:\d+p|Complete|COMPLETE)`),
	regexp.MustCompile(`(?i)\.S(\d{1,2})(?:\.|-|_|\s)Complete`),
	regexp.MustCompile(`(?i)\.Season\.(\d{1,2})\.`),
	regexp.MustCompile(`(?i)[/\\]Season\s*(\d{1,2})[/\\]`),
	regexp.MustCompile(`(?i)[/\\]S(\d{1,2})[/\\]`),
	regexp.MustCompile(`(?i)[-_\s]S(\d{1,2})[-_\s]`),
	regexp.MustCompile(`(?i)\.S(\d{1,2})(?:\.|-|_|\s)*$`),
	regexp.MustCompile(`(?i)Season\s*(\d{1,2})(?:[/\\]|$)`),
	regexp.MustCompile(`(?i)Temporada\s*(\d{1,2})`),
	regexp.MustCompile(`(?i)Saison\s*(\d{1,2})`),
	regexp.MustCompile(`(?i)Staffel\s*(\d{1,2})`),
	regexp.MustCompile(`(?i)\.S(\d{1,2})$`),
	regexp.MustCompile(`(?i)Season\s*(\d{1,2})$`),
	regexp.MustCompile(`(?i)S(\d{1,2})$`),
}

var episodePattern = regexp.MustCompile(`(?i)S\d{1,2}E(\d{1,3})`)
var multiEpisodePattern = regexp.MustCompile(`(?i)S\d{1,2}E(\d{1,3})-?E?(\d{1,3})`)

var videoExtensions = map[string]bool{
	".mkv": true,
	".mp4": true,
	".avi": true,
	".mov": true,
	".wmv": true,
	".flv": true,
}

func AnalyzeSeasonPack(files []types.EntryFile) *display.SeasonPackInfo {
	if len(files) == 0 {
		return &display.SeasonPackInfo{IsSeasonPack: false}
	}

	dirPath := filepath.Dir(files[0].Path)
	season := detectSeasonNumber(dirPath)

	if season == 0 && len(files) > 1 {
		var loopMax int
		if minIntFunc != nil {
			loopMax = minIntFunc(5, len(files))
		} else {
			loopMax = min(5, len(files))
		}

		for i := 0; i < loopMax; i++ {
			if s, _ := extractSeasonEpisode(filepath.Base(files[i].Path)); s > 0 {
				season = s
				break
			}
		}
	}

	if season == 0 {
		return &display.SeasonPackInfo{IsSeasonPack: false}
	}

	info := &display.SeasonPackInfo{
		IsSeasonPack: true,
		Season:       season,
		Episodes:     make([]int, 0),
	}

	episodeMap := make(map[int]bool)
	for _, file := range files {
		ext := strings.ToLower(filepath.Ext(file.Path))
		if videoExtensions[ext] {
			info.VideoFileCount++

			_, episode := extractSeasonEpisode(filepath.Base(file.Path))
			if episode > 0 {
				episodeMap[episode] = true
				if episode > info.MaxEpisode {
					info.MaxEpisode = episode
				}
			}

			multiEps := extractMultiEpisodes(filepath.Base(file.Path))
			for _, ep := range multiEps {
				if ep > 0 {
					episodeMap[ep] = true
					if ep > info.MaxEpisode {
						info.MaxEpisode = ep
					}
				}
			}
		}
	}

	for ep := range episodeMap {
		if ep > 0 {
			info.Episodes = append(info.Episodes, ep)
		}
	}
	sort.Ints(info.Episodes)

	if info.MaxEpisode > 0 {
		episodeCount := len(info.Episodes)

		expectedEpisodes := info.MaxEpisode

		info.MissingEpisodes = []int{}
		for i := 1; i <= info.MaxEpisode; i++ {
			if !episodeMap[i] {
				info.MissingEpisodes = append(info.MissingEpisodes, i)
			}
		}

		if episodeCount < expectedEpisodes {
			missingCount := expectedEpisodes - episodeCount
			percentMissing := float64(missingCount) / float64(expectedEpisodes) * 100

			if (missingCount >= 3 && info.MaxEpisode >= 7) || percentMissing > 50 {
				info.IsSuspicious = true
			}
		}
	}

	return info
}

func detectSeasonNumber(path string) int {
	for _, pattern := range seasonPackPatterns {
		matches := pattern.FindStringSubmatch(path)
		if len(matches) > 1 {
			seasonStr := matches[1]
			season, err := strconv.Atoi(seasonStr)
			if err != nil {
				return 0 // or log the error
			}
			return season
		}
	}
	return 0
}

func extractSeasonEpisode(filename string) (season, episode int) {
	epMatches := episodePattern.FindStringSubmatch(filename)
	if len(epMatches) > 1 {
		episodeStr := epMatches[1]
		var err error
		episode, err = strconv.Atoi(episodeStr)
		if err != nil {
			episode = 0
		}
	}

	seasonPattern := regexp.MustCompile(`(?i)S(\d{1,2})`)
	sMatches := seasonPattern.FindStringSubmatch(filename)
	if len(sMatches) > 1 {
		seasonStr := sMatches[1]
		var err error
		season, err = strconv.Atoi(seasonStr)
		if err != nil {
			season = 0
		}
	}

	return season, episode
}

func extractMultiEpisodes(filename string) []int {
	episodes := []int{}

	matches := multiEpisodePattern.FindStringSubmatch(filename)
	if len(matches) > 2 {
		startStr := matches[1]
		endStr := matches[2]

		start, err1 := strconv.Atoi(startStr)
		end, err2 := strconv.Atoi(endStr)

		if err1 == nil && err2 == nil && end >= start {
			for i := start; i <= end; i++ {
				episodes = append(episodes, i)
			}
		}
	}

	return episodes
}

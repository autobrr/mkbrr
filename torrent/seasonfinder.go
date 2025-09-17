package torrent

import (
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type SeasonPackInfo struct {
	Episodes        []int
	MissingEpisodes []int
	Season          int
	MaxEpisode      int
	VideoFileCount  int
	IsSeasonPack    bool
	IsSuspicious    bool
}

var seasonPackPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\.S(\d{1,2})(?:\.|-|_|\s)Complete`),
	regexp.MustCompile(`(?i)\.Season\.(\d{1,2})\.`),
	regexp.MustCompile(`(?i)\.S(\d{1,2})(?:\.|-|_|\s)*$`),
	regexp.MustCompile(`(?i)[-_\s]S(\d{1,2})[-_\s]`),
	regexp.MustCompile(`(?i)[/\\]Season\s*(\d{1,2})[/\\]`),
	regexp.MustCompile(`(?i)[/\\]S(\d{1,2})[/\\]`),
	regexp.MustCompile(`(?i)\.S(\d{1,2})\.(?:\d+p|Complete|COMPLETE)`),
	regexp.MustCompile(`(?i)Season\s*(\d{1,2})(?:[/\\]|$)`),
	regexp.MustCompile(`(?i)\.S(\d{1,2})$`),
}

var episodePattern = regexp.MustCompile(`(?i)S\d{1,2}E(\d{1,3})`)
var multiEpisodePattern = regexp.MustCompile(`(?i)S\d{1,2}E(\d{1,3})(?:-E?|E)(\d{1,3})`)

var videoExtensions = map[string]bool{
	".mkv": true,
	".mp4": true,
}

func AnalyzeSeasonPack(files []fileEntry) *SeasonPackInfo {
	if len(files) == 0 {
		return &SeasonPackInfo{IsSeasonPack: false}
	}

	dirPath := filepath.Dir(files[0].path)
	season := detectSeasonNumber(dirPath)

	if season == 0 && len(files) > 1 {
		for i := 0; i < minInt(5, len(files)); i++ {
			if s, _ := extractSeasonEpisode(filepath.Base(files[i].path)); s > 0 {
				season = s
				break
			}
		}
	}

	if season == 0 {
		return &SeasonPackInfo{IsSeasonPack: false}
	}

	info := &SeasonPackInfo{
		IsSeasonPack: false, // Will be set to true only if multiple episodes found
		Season:       season,
		Episodes:     make([]int, 0),
	}

	episodeMap := make(map[int]bool)
	for _, file := range files {
		ext := strings.ToLower(filepath.Ext(file.path))
		if videoExtensions[ext] {
			info.VideoFileCount++

			// check for multi-episodes first
			multiEps := extractMultiEpisodes(filepath.Base(file.path))
			if len(multiEps) > 0 {
				for _, ep := range multiEps {
					if ep > 0 {
						episodeMap[ep] = true
						if ep > info.MaxEpisode {
							info.MaxEpisode = ep
						}
					}
				}
			} else {
				_, episode := extractSeasonEpisode(filepath.Base(file.path))
				if episode > 0 {
					episodeMap[episode] = true
					if episode > info.MaxEpisode {
						info.MaxEpisode = episode
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

	// Only consider it a season pack if we have multiple episodes
	if len(info.Episodes) > 1 {
		info.IsSeasonPack = true
	}

	if info.MaxEpisode > 0 && info.IsSeasonPack {
		episodeCount := len(info.Episodes)

		expectedEpisodes := info.MaxEpisode

		info.MissingEpisodes = []int{}
		for i := 1; i <= info.MaxEpisode; i++ {
			if !episodeMap[i] {
				info.MissingEpisodes = append(info.MissingEpisodes, i)
			}
		}

		if episodeCount < expectedEpisodes {
			info.IsSuspicious = true
		}
	}

	return info
}

func detectSeasonNumber(path string) int {
	for _, pattern := range seasonPackPatterns {
		matches := pattern.FindStringSubmatch(path)
		if len(matches) > 1 {
			if season, err := strconv.Atoi(matches[1]); err == nil {
				return season
			}
		}
	}
	return 0
}

func extractSeasonEpisode(filename string) (season, episode int) {
	epMatches := episodePattern.FindStringSubmatch(filename)
	if len(epMatches) > 1 {
		episode, _ = strconv.Atoi(epMatches[1])
	}

	seasonPattern := regexp.MustCompile(`(?i)S(\d{1,2})`)
	sMatches := seasonPattern.FindStringSubmatch(filename)
	if len(sMatches) > 1 {
		season, _ = strconv.Atoi(sMatches[1])
	}

	return season, episode
}

func extractMultiEpisodes(filename string) []int {
	episodes := []int{}

	matches := multiEpisodePattern.FindStringSubmatch(filename)

	if len(matches) > 2 {
		start, err1 := strconv.Atoi(matches[1])
		end, err2 := strconv.Atoi(matches[2])
		if err1 == nil && err2 == nil && end >= start {
			for i := start; i <= end; i++ {
				episodes = append(episodes, i)
			}
		}
	}

	return episodes
}

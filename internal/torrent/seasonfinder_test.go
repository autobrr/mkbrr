package torrent

import (
	"path/filepath"
	"testing"
)

func TestDetectSeasonNumber(t *testing.T) {
	tests := []struct {
		path     string
		expected int
	}{
		{filepath.Join("/test", "Dexter.Original.Sin.S01.1080p"), 1},
		{filepath.Join("/test", "Show.Name.S02.Complete"), 2},
		{filepath.Join("/test", "Some.Show.Season.03.1080p"), 3},
		{filepath.Join("/test", "My.Show.S04"), 4},
		{filepath.Join("/test", "Season 05"), 5},
		{filepath.Join("/test", "Regular.Movie.2024"), 0},
	}

	for _, tc := range tests {
		season := detectSeasonNumber(tc.path)
		if season != tc.expected {
			t.Errorf("Expected season %d for path %s, got %d", tc.expected, tc.path, season)
		}
	}
}

func TestExtractSeasonEpisode(t *testing.T) {
	tests := []struct {
		filename      string
		expectSeason  int
		expectEpisode int
	}{
		{"Show.S01E01.Name.mkv", 1, 1},
		{"S02E05.Episode.Name.mp4", 2, 5},
		{"My.Show.S03E10.1080p.mkv", 3, 10},
		{"Movie.2024.mkv", 0, 0}, // Not an episode
	}

	for _, tc := range tests {
		season, episode := extractSeasonEpisode(tc.filename)
		if season != tc.expectSeason || episode != tc.expectEpisode {
			t.Errorf("For %s expected S%02dE%02d, got S%02dE%02d",
				tc.filename, tc.expectSeason, tc.expectEpisode, season, episode)
		}
	}
}

func TestMultipleEpisodes(t *testing.T) {
	tests := []struct {
		filename         string
		expectedEpisodes []int
	}{
		{"Show.S01E01E02.mkv", []int{1, 2}},
		{"Show.S01E05-E07.mkv", []int{5, 6, 7}},
	}

	for _, tc := range tests {
		episodes := extractMultiEpisodes(tc.filename)

		if len(episodes) != len(tc.expectedEpisodes) {
			t.Errorf("For %s expected %v episodes, got %v", tc.filename, tc.expectedEpisodes, episodes)
			continue
		}

		for i, ep := range episodes {
			if i < len(tc.expectedEpisodes) && ep != tc.expectedEpisodes[i] {
				t.Errorf("For %s expected episode %d at position %d, got %d",
					tc.filename, tc.expectedEpisodes[i], i, ep)
			}
		}
	}
}

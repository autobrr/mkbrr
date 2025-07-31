package torrent

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
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
		t.Run(tc.path, func(t *testing.T) {
			season := detectSeasonNumber(tc.path)
			assert.Equal(t, tc.expected, season, "Season number should match for path %s", tc.path)
		})
	}
}

func TestAnalyzeSeasonPack_MultiEpisode(t *testing.T) {
	files := []fileEntry{
		{path: filepath.Join("/test", "Show.S02E01E02.mkv")},
		{path: filepath.Join("/test", "Show.S02E03.mkv")},
		{path: filepath.Join("/test", "Show.S02E04.mkv")},
		{path: filepath.Join("/test", "Show.S02E05.mkv")},
		{path: filepath.Join("/test", "Show.S02E06.mkv")},
		{path: filepath.Join("/test", "Show.S02E07.mkv")},
		{path: filepath.Join("/test", "Show.S02E08.mkv")},
		{path: filepath.Join("/test", "Show.S02E09.mkv")},
		{path: filepath.Join("/test", "Show.S02E10E11.mkv")},
		{path: filepath.Join("/test", "Show.S02E12.mkv")},
	}

	info := AnalyzeSeasonPack(files)

	assert.True(t, info.IsSeasonPack, "Should be detected as season pack")
	assert.Equal(t, 2, info.Season, "Should be Season 2")
	assert.Equal(t, 10, info.VideoFileCount, "Should have 10 video files")
	assert.Equal(t, 12, info.MaxEpisode, "Maximum episode should be 12")
	assert.Len(t, info.Episodes, 12, "Should have 12 unique episodes")
	assert.Empty(t, info.MissingEpisodes, "Should have no missing episodes")
	assert.False(t, info.IsSuspicious, "Complete season pack should not be suspicious")

	expectedEpisodes := make([]int, 12)
	for i := range expectedEpisodes {
		expectedEpisodes[i] = i + 1
	}
	assert.Equal(t, expectedEpisodes, info.Episodes, "Episodes should match expected sequence")
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
		t.Run(tc.filename, func(t *testing.T) {
			season, episode := extractSeasonEpisode(tc.filename)
			assert.Equal(t, tc.expectSeason, season, "Season should match for %s", tc.filename)
			assert.Equal(t, tc.expectEpisode, episode, "Episode should match for %s", tc.filename)
		})
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
		t.Run(tc.filename, func(t *testing.T) {
			episodes := extractMultiEpisodes(tc.filename)
			assert.Equal(t, tc.expectedEpisodes, episodes, "Episodes should match for %s", tc.filename)
		})
	}
}

func TestAnalyzeSeasonPack_SingleEpisode(t *testing.T) {
	tests := []struct {
		name  string
		files []fileEntry
	}{
		{
			name: "Single episode S01E10",
			files: []fileEntry{
				{path: filepath.Join("/test", "Screens", "ShowName.S01E10.720p.x264-Group.Screen0001.png")},
				{path: filepath.Join("/test", "Screens", "ShowName.S01E10.720p.x264-Group.Screen0002.png")},
				{path: filepath.Join("/test", "Screens", "ShowName.S01E10.720p.x264-Group.Screen0003.png")},
				{path: filepath.Join("/test", "Screens", "ShowName.S01E10.720p.x264-Group.Screen0004.png")},
				{path: filepath.Join("/test", "ShowName.S01E10.720p.x264-Group.mkv")},
				{path: filepath.Join("/test", "ShowName.S01E10.720p.x264-Group.nfo")},
			},
		},
		{
			name: "Single episode S02E05",
			files: []fileEntry{
				{path: filepath.Join("/test", "Show.S02E05.1080p.mkv")},
				{path: filepath.Join("/test", "Show.S02E05.1080p.nfo")},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			info := AnalyzeSeasonPack(tc.files)

			assert.False(t, info.IsSeasonPack, "Expected IsSeasonPack to be false for single episode")
			assert.Empty(t, info.MissingEpisodes, "Expected no missing episodes for single episode")
			assert.False(t, info.IsSuspicious, "Expected IsSuspicious to be false for single episode")
		})
	}
}

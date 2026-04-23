package torrent

import (
	"testing"
)

// TestNormalizePattern tests the normalizePattern function which converts
// patterns to doublestar format for consistent matching.
func TestNormalizePattern(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		// Simple filename patterns should be prefixed with **/
		{"*.nfo", "**/*.nfo"},
		{"*.mkv", "**/*.mkv"},
		{"thumbs.db", "**/thumbs.db"},

		// Patterns with path separators should remain unchanged
		{"**/extras/**", "**/extras/**"},
		{"Season1/Subs/*.srt", "Season1/Subs/*.srt"},
		{"foo/bar/*.txt", "foo/bar/*.txt"},

		// Patterns already starting with ** should remain unchanged
		{"**/*.mkv", "**/*.mkv"},
		{"**/sample/**", "**/sample/**"},

		// Backslashes should be normalized to forward slashes
		{"Season1\\Subs\\*.srt", "Season1/Subs/*.srt"},

		// Empty and whitespace
		{"", ""},
		{"  *.nfo  ", "**/*.nfo"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := normalizePattern(tt.input); got != tt.want {
				t.Errorf("normalizePattern(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestSplitPatterns tests the splitPatterns function which splits
// comma-separated patterns while respecting brace expressions.
func TestSplitPatterns(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		// Simple comma-separated
		{"*.nfo,*.txt", []string{"*.nfo", "*.txt"}},
		{"*.nfo, *.txt, *.jpg", []string{"*.nfo", "*.txt", "*.jpg"}},

		// Brace alternatives should NOT be split
		{"*.{nfo,txt,jpg}", []string{"*.{nfo,txt,jpg}"}},
		{"*.{mkv,mp4,avi},*.srt", []string{"*.{mkv,mp4,avi}", "*.srt"}},

		// Nested braces
		{"*.{a,{b,c}}", []string{"*.{a,{b,c}}"}},

		// Mixed
		{"**/extras/**,*.{nfo,txt}", []string{"**/extras/**", "*.{nfo,txt}"}},

		// Single pattern
		{"*.nfo", []string{"*.nfo"}},

		// Empty and whitespace
		{"", []string{}},
		{"  ", []string{}},
		{"*.nfo,  ,*.txt", []string{"*.nfo", "*.txt"}},

		// Trailing/leading commas
		{",*.nfo,*.txt,", []string{"*.nfo", "*.txt"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := splitPatterns(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("splitPatterns(%q) = %v, want %v", tt.input, got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("splitPatterns(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

// TestMatchPattern tests the matchPattern function which matches glob patterns
// against paths using doublestar with case-insensitivity and directory handling.
func TestMatchPattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		relPath string
		isDir   bool
		want    bool
		wantErr bool
	}{
		// Simple extension patterns
		{
			name:    "simple extension - match at root",
			pattern: "*.nfo",
			relPath: "file.nfo",
			isDir:   false,
			want:    true,
		},
		{
			name:    "simple extension - match in subdir",
			pattern: "*.nfo",
			relPath: "Season1/file.nfo",
			isDir:   false,
			want:    true,
		},
		{
			name:    "simple extension - no match",
			pattern: "*.nfo",
			relPath: "file.mkv",
			isDir:   false,
			want:    false,
		},

		// Doublestar patterns
		{
			name:    "doublestar - match directory",
			pattern: "**/extras/**",
			relPath: "extras",
			isDir:   true,
			want:    true,
		},
		{
			name:    "doublestar - match nested directory",
			pattern: "**/extras/**",
			relPath: "Season1/extras",
			isDir:   true,
			want:    true,
		},
		{
			name:    "doublestar - match file in directory",
			pattern: "**/extras/**",
			relPath: "extras/behind_scenes.mkv",
			isDir:   false,
			want:    true,
		},
		{
			name:    "doublestar - match file in nested directory",
			pattern: "**/extras/**",
			relPath: "Season1/extras/behind_scenes.mkv",
			isDir:   false,
			want:    true,
		},

		// Specific path patterns
		{
			name:    "specific path - match",
			pattern: "Season1/Subs/*.srt",
			relPath: "Season1/Subs/english.srt",
			isDir:   false,
			want:    true,
		},
		{
			name:    "specific path - no match different season",
			pattern: "Season1/Subs/*.srt",
			relPath: "Season2/Subs/english.srt",
			isDir:   false,
			want:    false,
		},

		// Case insensitivity
		{
			name:    "case insensitive - uppercase path",
			pattern: "**/sample/**",
			relPath: "SAMPLE/trailer.mkv",
			isDir:   false,
			want:    true,
		},
		{
			name:    "case insensitive - mixed case",
			pattern: "**/Sample/**",
			relPath: "sample/trailer.mkv",
			isDir:   false,
			want:    true,
		},

		// Brace alternatives
		{
			name:    "brace alternatives - first option",
			pattern: "*.{nfo,txt,jpg}",
			relPath: "readme.nfo",
			isDir:   false,
			want:    true,
		},
		{
			name:    "brace alternatives - second option",
			pattern: "*.{nfo,txt,jpg}",
			relPath: "readme.txt",
			isDir:   false,
			want:    true,
		},
		{
			name:    "brace alternatives - no match",
			pattern: "*.{nfo,txt,jpg}",
			relPath: "movie.mkv",
			isDir:   false,
			want:    false,
		},

		// Dotfile patterns
		{
			name:    "dotfile directory",
			pattern: ".*/**",
			relPath: ".hidden",
			isDir:   true,
			want:    true,
		},
		{
			name:    "file in dotfile directory",
			pattern: ".*/**",
			relPath: ".git/config",
			isDir:   false,
			want:    true,
		},

		// Character classes
		{
			name:    "character class - match",
			pattern: "[Ss]ample*",
			relPath: "Sample.mkv",
			isDir:   false,
			want:    true,
		},
		{
			name:    "character class - match lowercase",
			pattern: "[Ss]ample*",
			relPath: "sample.mkv",
			isDir:   false,
			want:    true,
		},

		// Empty patterns
		{
			name:    "empty pattern",
			pattern: "",
			relPath: "file.txt",
			isDir:   false,
			want:    false,
		},
		{
			name:    "empty path",
			pattern: "*.txt",
			relPath: "",
			isDir:   false,
			want:    false,
		},

		// Invalid patterns that should return errors
		{
			name:    "invalid pattern - unclosed bracket",
			pattern: "[abc",
			relPath: "file.txt",
			isDir:   false,
			want:    false,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := matchPattern(tt.pattern, tt.relPath, tt.isDir)
			if (err != nil) != tt.wantErr {
				t.Errorf("matchPattern() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("matchPattern(%q, %q, %v) = %v, want %v", tt.pattern, tt.relPath, tt.isDir, got, tt.want)
			}
		})
	}
}

// TestShouldIgnoreEntry tests the shouldIgnoreEntry function which determines
// if a file or directory should be ignored based on include/exclude patterns.
func TestShouldIgnoreEntry(t *testing.T) {
	tests := []struct {
		name            string
		relPath         string
		isDir           bool
		excludePatterns []string
		includePatterns []string
		wantIgnore      bool
		wantErr         bool
	}{
		// Backward compatibility - simple extension patterns
		{
			name:            "simple extension exclude - file at root",
			relPath:         "file.nfo",
			isDir:           false,
			excludePatterns: []string{"*.nfo"},
			wantIgnore:      true,
		},
		{
			name:            "simple extension exclude - file in subdir",
			relPath:         "Season1/file.nfo",
			isDir:           false,
			excludePatterns: []string{"*.nfo"},
			wantIgnore:      true,
		},
		{
			name:            "simple extension include - match",
			relPath:         "movie.mkv",
			isDir:           false,
			includePatterns: []string{"*.mkv"},
			wantIgnore:      false,
		},
		{
			name:            "simple extension include - no match",
			relPath:         "movie.avi",
			isDir:           false,
			includePatterns: []string{"*.mkv"},
			wantIgnore:      true,
		},
		{
			name:            "simple extension include - match in subdir",
			relPath:         "Season1/episode1.mkv",
			isDir:           false,
			includePatterns: []string{"*.mkv"},
			wantIgnore:      false,
		},

		// Doublestar patterns
		{
			name:            "doublestar exclude - match directory",
			relPath:         "Season1/extras",
			isDir:           true,
			excludePatterns: []string{"**/extras/**"},
			wantIgnore:      true,
		},
		{
			name:            "doublestar exclude - match file in directory",
			relPath:         "Season1/extras/behind_scenes.mkv",
			isDir:           false,
			excludePatterns: []string{"**/extras/**"},
			wantIgnore:      true,
		},
		{
			name:            "doublestar exclude - no match",
			relPath:         "Season1/episode1.mkv",
			isDir:           false,
			excludePatterns: []string{"**/extras/**"},
			wantIgnore:      false,
		},

		// Specific path patterns
		{
			name:            "specific path exclude - match",
			relPath:         "Season1/Subs/english.srt",
			isDir:           false,
			excludePatterns: []string{"Season1/Subs/*.srt"},
			wantIgnore:      true,
		},
		{
			name:            "specific path exclude - no match different season",
			relPath:         "Season2/Subs/english.srt",
			isDir:           false,
			excludePatterns: []string{"Season1/Subs/*.srt"},
			wantIgnore:      false,
		},

		// Include patterns with directories
		{
			name:            "include patterns - directories always traversed",
			relPath:         "Season1",
			isDir:           true,
			includePatterns: []string{"*.mkv"},
			wantIgnore:      false, // Should traverse to find .mkv files
		},
		{
			name:            "include patterns - nested directories traversed",
			relPath:         "Season1/Subs",
			isDir:           true,
			includePatterns: []string{"*.mkv"},
			wantIgnore:      false, // Should still traverse
		},

		// Comma-separated patterns
		{
			name:            "comma-separated exclude - first pattern",
			relPath:         "file.nfo",
			isDir:           false,
			excludePatterns: []string{"*.nfo,*.txt"},
			wantIgnore:      true,
		},
		{
			name:            "comma-separated exclude - second pattern",
			relPath:         "readme.txt",
			isDir:           false,
			excludePatterns: []string{"*.nfo,*.txt"},
			wantIgnore:      true,
		},
		{
			name:            "comma-separated include - match",
			relPath:         "movie.mkv",
			isDir:           false,
			includePatterns: []string{"*.mkv,*.mp4,*.avi"},
			wantIgnore:      false,
		},

		// Multiple pattern groups
		{
			name:            "multiple pattern groups exclude",
			relPath:         "file.nfo",
			isDir:           false,
			excludePatterns: []string{"*.txt", "*.nfo"},
			wantIgnore:      true,
		},

		// Hardcoded ignores
		{
			name:            "hardcoded directory ignore - @eadir",
			relPath:         "video/@eaDir/thumb.jpg",
			isDir:           false,
			excludePatterns: nil,
			wantIgnore:      true,
		},
		{
			name:            "hardcoded file ignore - thumbs.db",
			relPath:         "folder/thumbs.db",
			isDir:           false,
			excludePatterns: nil,
			wantIgnore:      true,
		},
		{
			name:            "hardcoded file ignore - .ds_store",
			relPath:         ".DS_Store",
			isDir:           false,
			excludePatterns: nil,
			wantIgnore:      true,
		},
		{
			name:            "hardcoded file ignore - .torrent",
			relPath:         "movie.torrent",
			isDir:           false,
			excludePatterns: nil,
			wantIgnore:      true,
		},

		// Case insensitivity
		{
			name:            "case insensitive - uppercase extension",
			relPath:         "file.NFO",
			isDir:           false,
			excludePatterns: []string{"*.nfo"},
			wantIgnore:      true,
		},
		{
			name:            "case insensitive - mixed case path",
			relPath:         "SAMPLE/trailer.MKV",
			isDir:           false,
			excludePatterns: []string{"**/sample/**"},
			wantIgnore:      true,
		},

		// Root path handling
		{
			name:            "empty path - should not ignore",
			relPath:         "",
			isDir:           true,
			excludePatterns: []string{"*"},
			wantIgnore:      false,
		},
		{
			name:            "dot path - should not ignore",
			relPath:         ".",
			isDir:           true,
			excludePatterns: []string{"*"},
			wantIgnore:      false,
		},

		// Brace alternatives
		{
			name:            "brace alternatives exclude",
			relPath:         "readme.txt",
			isDir:           false,
			excludePatterns: []string{"*.{nfo,txt,jpg}"},
			wantIgnore:      true,
		},
		{
			name:            "include match keeps file even when exclude also matches",
			relPath:         "sample.mkv",
			isDir:           false,
			excludePatterns: []string{"sample*"},
			includePatterns: []string{"*.mkv"},
			wantIgnore:      false,
		},

		// Regression: pattern with specific file should NOT skip entire directory
		{
			name:            "specific file pattern should not skip directory",
			relPath:         "foo/extras",
			isDir:           true,
			excludePatterns: []string{"**/extras/specific.txt"},
			wantIgnore:      false, // Should NOT skip - only specific.txt should be excluded
		},
		{
			name:            "specific file pattern should exclude the file",
			relPath:         "foo/extras/specific.txt",
			isDir:           false,
			excludePatterns: []string{"**/extras/specific.txt"},
			wantIgnore:      true, // Should exclude the actual file
		},
		{
			name:            "specific file pattern should not exclude other files",
			relPath:         "foo/extras/other.txt",
			isDir:           false,
			excludePatterns: []string{"**/extras/specific.txt"},
			wantIgnore:      false, // Should NOT exclude other files in extras
		},

		// No patterns - should keep
		{
			name:       "no patterns - keep file",
			relPath:    "movie.mkv",
			isDir:      false,
			wantIgnore: false,
		},
		{
			name:       "no patterns - keep directory",
			relPath:    "Season1",
			isDir:      true,
			wantIgnore: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := shouldIgnoreEntry(tt.relPath, tt.isDir, tt.excludePatterns, tt.includePatterns)
			if (err != nil) != tt.wantErr {
				t.Errorf("shouldIgnoreEntry() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.wantIgnore {
				t.Errorf("shouldIgnoreEntry(%q, isDir=%v, exclude=%v, include=%v) = %v, want %v",
					tt.relPath, tt.isDir, tt.excludePatterns, tt.includePatterns, got, tt.wantIgnore)
			}
		})
	}
}

// TestShouldIgnoreFile tests the deprecated shouldIgnoreFile wrapper function
// for backward compatibility with filename-only matching.
func TestShouldIgnoreFile(t *testing.T) {
	// Test backward compatibility of shouldIgnoreFile wrapper
	tests := []struct {
		name            string
		path            string
		excludePatterns []string
		includePatterns []string
		wantIgnore      bool
	}{
		{
			name:            "backward compat - exclude by extension",
			path:            "/some/path/file.nfo",
			excludePatterns: []string{"*.nfo"},
			wantIgnore:      true,
		},
		{
			name:            "backward compat - include by extension",
			path:            "/some/path/movie.mkv",
			includePatterns: []string{"*.mkv"},
			wantIgnore:      false,
		},
		{
			name:            "backward compat - no match include",
			path:            "/some/path/movie.avi",
			includePatterns: []string{"*.mkv"},
			wantIgnore:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := shouldIgnoreFile(tt.path, tt.excludePatterns, tt.includePatterns)
			if err != nil {
				t.Errorf("shouldIgnoreFile() error = %v", err)
				return
			}
			if got != tt.wantIgnore {
				t.Errorf("shouldIgnoreFile(%q, %v, %v) = %v, want %v",
					tt.path, tt.excludePatterns, tt.includePatterns, got, tt.wantIgnore)
			}
		})
	}
}

// TestShouldIgnoreDir tests the shouldIgnoreDir function which checks
// if any path segment matches the hardcoded ignored directory names.
func TestShouldIgnoreDir(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		wantIgnore bool
	}{
		{
			name:       "should ignore @eadir",
			path:       "/some/path/@eaDir/file",
			wantIgnore: true,
		},
		{
			name:       "should ignore @eadir case insensitive",
			path:       "/some/path/@EADIR/file",
			wantIgnore: true,
		},
		{
			name:       "should not ignore regular dir",
			path:       "/some/path/normal/file",
			wantIgnore: false,
		},
		{
			name:       "should handle windows paths",
			path:       "C:\\some\\path\\@eaDir\\file",
			wantIgnore: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldIgnoreDir(tt.path); got != tt.wantIgnore {
				t.Errorf("shouldIgnoreDir(%q) = %v, want %v", tt.path, got, tt.wantIgnore)
			}
		})
	}
}

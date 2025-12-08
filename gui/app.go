package main

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/autobrr/mkbrr/internal/preset"
	"github.com/autobrr/mkbrr/internal/trackers"
	"github.com/autobrr/mkbrr/torrent"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct holds the application state
type App struct {
	ctx     context.Context
	version string
}

// NewApp creates a new App application struct
func NewApp(version string) *App {
	return &App{version: version}
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// ProgressEvent represents progress data sent to the frontend
type ProgressEvent struct {
	Completed int     `json:"completed"`
	Total     int     `json:"total"`
	HashRate  float64 `json:"hashRate"`
	Percent   float64 `json:"percent"`
}

// CreateRequest represents a torrent creation request from the frontend.
//
// Required fields:
//   - Path: The source file or directory to create a torrent from
//
// Optional fields (all others): Have sensible defaults if not specified
type CreateRequest struct {
	Path            string   `json:"path"`            // Required: source file/directory path
	Name            string   `json:"name"`            // Optional: override torrent name (defaults to source name)
	TrackerURLs     []string `json:"trackerUrls"`     // Optional: tracker announce URLs
	WebSeeds        []string `json:"webSeeds"`        // Optional: web seed URLs
	Comment         string   `json:"comment"`         // Optional: torrent comment
	Source          string   `json:"source"`          // Optional: source tag
	IsPrivate       *bool    `json:"isPrivate"`       // Optional: private flag (nil = true)
	PieceLengthExp  uint     `json:"pieceLengthExp"`  // Optional: piece length as 2^exp (0 = auto)
	MaxPieceLength  uint     `json:"maxPieceLength"`  // Optional: max piece length as 2^exp
	OutputPath      string   `json:"outputPath"`      // Optional: full output path (mutually exclusive with OutputDir)
	OutputDir       string   `json:"outputDir"`       // Optional: output directory (defaults to source dir)
	NoDate          bool     `json:"noDate"`          // Optional: exclude creation date
	NoCreator       bool     `json:"noCreator"`       // Optional: exclude creator string
	Entropy         bool     `json:"entropy"`         // Optional: add random entropy for unique hash
	SkipPrefix      bool     `json:"skipPrefix"`      // Optional: don't prefix output filename
	ExcludePatterns []string `json:"excludePatterns"` // Optional: file exclusion patterns
	IncludePatterns []string `json:"includePatterns"` // Optional: file inclusion patterns
	PresetName      string   `json:"presetName"`      // Optional: preset name to apply
	PresetFile      string   `json:"presetFile"`      // Optional: path to preset file
}

// TorrentResult represents the result of torrent creation
type TorrentResult struct {
	Path       string `json:"path"`
	InfoHash   string `json:"infoHash"`
	Size       int64  `json:"size"`
	PieceCount int    `json:"pieceCount"`
	FileCount  int    `json:"fileCount"`
	Warning    string `json:"warning,omitempty"`
}

// InspectResult represents torrent metadata for inspection
type InspectResult struct {
	Name         string     `json:"name"`
	InfoHash     string     `json:"infoHash"`
	Size         int64      `json:"size"`
	PieceLength  int64      `json:"pieceLength"`
	PieceCount   int        `json:"pieceCount"`
	Trackers     []string   `json:"trackers"`
	WebSeeds     []string   `json:"webSeeds"`
	IsPrivate    bool       `json:"isPrivate"`
	Source       string     `json:"source"`
	Comment      string     `json:"comment"`
	CreatedBy    string     `json:"createdBy"`
	CreationDate int64      `json:"creationDate"`
	FileCount    int        `json:"fileCount"`
	Files        []FileInfo `json:"files"`
}

// FileInfo represents a file in a torrent
type FileInfo struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

// VerifyRequest represents a verification request.
//
// Required fields:
//   - TorrentPath: Path to the .torrent file to verify against
//   - ContentPath: Path to the content (file or directory) to verify
type VerifyRequest struct {
	TorrentPath string `json:"torrentPath"` // Required: path to .torrent file
	ContentPath string `json:"contentPath"` // Required: path to content to verify
}

// VerifyResult represents verification results
type VerifyResult struct {
	Completion   float64  `json:"completion"`
	TotalPieces  int      `json:"totalPieces"`
	GoodPieces   int      `json:"goodPieces"`
	BadPieces    int      `json:"badPieces"`
	MissingFiles []string `json:"missingFiles"`
}

// ModifyRequest represents a torrent modification request.
//
// Required fields:
//   - TorrentPath: Path to the .torrent file to modify
//
// Optional fields (all others): Only non-empty/non-nil values will be applied
type ModifyRequest struct {
	TorrentPath   string   `json:"torrentPath"`   // Required: path to .torrent file to modify
	TrackerURLs   []string `json:"trackerUrls"`   // Optional: new tracker URLs (replaces existing)
	WebSeeds      []string `json:"webSeeds"`      // Optional: new web seed URLs
	Comment       string   `json:"comment"`       // Optional: new comment
	Source        string   `json:"source"`        // Optional: new source tag
	IsPrivate     *bool    `json:"isPrivate"`     // Optional: set private flag (nil = unchanged)
	NoDate        bool     `json:"noDate"`        // Optional: remove creation date
	NoCreator     bool     `json:"noCreator"`     // Optional: remove creator string
	Entropy       bool     `json:"entropy"`       // Optional: add entropy for unique hash
	SkipPrefix    bool     `json:"skipPrefix"`    // Optional: don't prefix output filename
	OutputDir     string   `json:"outputDir"`     // Optional: output directory for modified file
	OutputPattern string   `json:"outputPattern"` // Optional: output filename pattern
	PresetName    string   `json:"presetName"`    // Optional: preset to apply
	PresetFile    string   `json:"presetFile"`    // Optional: path to preset file
	DryRun        bool     `json:"dryRun"`        // Optional: simulate modification without writing
}

// ModifyResult represents the result of torrent modification
type ModifyResult struct {
	OutputPath  string `json:"outputPath"`
	WasModified bool   `json:"wasModified"`
}

// TrackerInfo represents tracker-specific information
type TrackerInfo struct {
	MaxPieceLength uint   `json:"maxPieceLength"`
	MaxTorrentSize uint64 `json:"maxTorrentSize"`
	DefaultSource  string `json:"defaultSource"`
	HasCustomRules bool   `json:"hasCustomRules"`
}

// PresetInfo represents a preset configuration
type PresetInfo struct {
	Name    string          `json:"name"`
	Options *preset.Options `json:"options"`
}

// PresetsResult represents all presets with any loading errors
type PresetsResult struct {
	Presets map[string]*preset.Options `json:"presets"`
	Errors  []string                   `json:"errors,omitempty"`
}

// === File Dialogs ===

// SelectPath opens a native directory picker
func (a *App) SelectPath() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Content Directory or File",
	})
}

// SelectFile opens a native file picker
func (a *App) SelectFile() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select File",
	})
}

// SelectTorrentFile opens a native file picker for .torrent files
func (a *App) SelectTorrentFile() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Torrent File",
		Filters: []runtime.FileFilter{
			{DisplayName: "Torrent Files", Pattern: "*.torrent"},
		},
	})
}

// SelectMultipleTorrentFiles opens a native file picker for multiple .torrent files
func (a *App) SelectMultipleTorrentFiles() ([]string, error) {
	return runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Torrent Files",
		Filters: []runtime.FileFilter{
			{DisplayName: "Torrent Files", Pattern: "*.torrent"},
		},
	})
}

// === Create Operations ===

// CreateTorrent creates a new torrent file
func (a *App) CreateTorrent(req CreateRequest) (*TorrentResult, error) {
	var pieceLengthExp *uint
	if req.PieceLengthExp > 0 {
		pieceLengthExp = &req.PieceLengthExp
	}

	var maxPieceLength *uint
	if req.MaxPieceLength > 0 {
		maxPieceLength = &req.MaxPieceLength
	}

	// Default output directory to source directory for GUI
	outputDir := req.OutputDir
	if outputDir == "" && req.OutputPath == "" {
		outputDir = filepath.Dir(req.Path)
	}

	// Handle IsPrivate pointer - default to true if not specified
	isPrivate := true
	if req.IsPrivate != nil {
		isPrivate = *req.IsPrivate
	}

	opts := torrent.CreateOptions{
		Path:            req.Path,
		Name:            req.Name,
		TrackerURLs:     req.TrackerURLs,
		WebSeeds:        req.WebSeeds,
		Comment:         req.Comment,
		Source:          req.Source,
		IsPrivate:       isPrivate,
		PieceLengthExp:  pieceLengthExp,
		MaxPieceLength:  maxPieceLength,
		OutputPath:      req.OutputPath,
		OutputDir:       outputDir,
		NoDate:          req.NoDate,
		NoCreator:       req.NoCreator,
		Entropy:         req.Entropy,
		SkipPrefix:      req.SkipPrefix,
		ExcludePatterns: req.ExcludePatterns,
		IncludePatterns: req.IncludePatterns,
		Quiet:           true, // Suppress CLI output
		ProgressCallback: func(completed, total int, hashRate float64) {
			if a.ctx == nil {
				return
			}
			percent := 0.0
			if total > 0 {
				percent = float64(completed) / float64(total) * 100
			}
			runtime.EventsEmit(a.ctx, "create:progress", ProgressEvent{
				Completed: completed,
				Total:     total,
				HashRate:  hashRate / (1024 * 1024), // Convert bytes/s to MB/s
				Percent:   percent,
			})
		},
	}

	// Load preset if specified
	if req.PresetName != "" {
		presetOpts, err := preset.LoadPresetOptions(req.PresetFile, req.PresetName)
		if err != nil {
			return nil, fmt.Errorf("failed to load preset: %w", err)
		}
		applyPresetToCreateOptions(&opts, presetOpts)
	}

	// Use the high-level Create function which returns TorrentInfo
	info, err := torrent.Create(opts)
	if err != nil {
		return nil, err
	}

	// Read back the created torrent to get accurate piece/file counts
	pieceCount := 0
	fileCount := 1
	size := info.Size
	var warning string

	t, err := torrent.LoadFromFile(info.Path)
	if err != nil {
		log.Printf("Warning: failed to re-read created torrent for metadata: %v", err)
		warning = fmt.Sprintf("Created torrent but failed to verify metadata: %v", err)
	} else {
		mi := t.GetInfo()
		pieceCount = mi.NumPieces()
		size = mi.TotalLength()
		if len(mi.Files) > 0 {
			fileCount = len(mi.Files)
		}
	}

	return &TorrentResult{
		Path:       info.Path,
		InfoHash:   info.InfoHash,
		Size:       size,
		PieceCount: pieceCount,
		FileCount:  fileCount,
		Warning:    warning,
	}, nil
}

// === Inspect Operations ===

// InspectTorrent loads and returns torrent metadata
func (a *App) InspectTorrent(path string) (*InspectResult, error) {
	t, err := torrent.LoadFromFile(path)
	if err != nil {
		return nil, err
	}

	// Get info using the GetInfo method
	info := t.GetInfo()

	// Collect trackers
	var trackerList []string
	if t.Announce != "" {
		trackerList = append(trackerList, t.Announce)
	}
	for _, tier := range t.AnnounceList {
		for _, tr := range tier {
			if tr != t.Announce {
				trackerList = append(trackerList, tr)
			}
		}
	}

	// Collect files
	var files []FileInfo
	if len(info.Files) > 0 {
		for _, f := range info.Files {
			files = append(files, FileInfo{
				Path: filepath.Join(f.Path...),
				Size: f.Length,
			})
		}
	} else {
		files = append(files, FileInfo{
			Path: info.Name,
			Size: info.Length,
		})
	}

	// Compute info hash
	infoHash := t.HashInfoBytes().String()

	return &InspectResult{
		Name:         info.Name,
		InfoHash:     infoHash,
		Size:         info.TotalLength(),
		PieceLength:  info.PieceLength,
		PieceCount:   info.NumPieces(),
		Trackers:     trackerList,
		WebSeeds:     t.UrlList,
		IsPrivate:    info.Private != nil && *info.Private,
		Source:       info.Source,
		Comment:      t.Comment,
		CreatedBy:    t.CreatedBy,
		CreationDate: t.CreationDate,
		FileCount:    len(files),
		Files:        files,
	}, nil
}

// === Verify Operations ===

// VerifyTorrent verifies torrent data against local files
func (a *App) VerifyTorrent(req VerifyRequest) (*VerifyResult, error) {
	opts := torrent.VerifyOptions{
		TorrentPath: req.TorrentPath,
		ContentPath: req.ContentPath,
		Quiet:       true,
		ProgressCallback: func(completed, total int, hashRate float64) {
			if a.ctx == nil {
				return
			}
			percent := 0.0
			if total > 0 {
				percent = float64(completed) / float64(total) * 100
			}
			runtime.EventsEmit(a.ctx, "verify:progress", ProgressEvent{
				Completed: completed,
				Total:     total,
				HashRate:  hashRate, // Already in MiB/s from torrent package
				Percent:   percent,
			})
		},
	}

	result, err := torrent.VerifyData(opts)
	if err != nil {
		return nil, err
	}

	return &VerifyResult{
		Completion:   result.Completion,
		TotalPieces:  result.TotalPieces,
		GoodPieces:   result.GoodPieces,
		BadPieces:    result.BadPieces,
		MissingFiles: result.MissingFiles,
	}, nil
}

// === Modify Operations ===

// ModifyTorrent modifies an existing torrent file
func (a *App) ModifyTorrent(req ModifyRequest) (*ModifyResult, error) {
	opts := torrent.ModifyOptions{
		TrackerURLs:   req.TrackerURLs,
		WebSeeds:      req.WebSeeds,
		Comment:       req.Comment,
		Source:        req.Source,
		IsPrivate:     req.IsPrivate,
		NoDate:        req.NoDate,
		NoCreator:     req.NoCreator,
		Entropy:       req.Entropy,
		SkipPrefix:    req.SkipPrefix,
		OutputDir:     req.OutputDir,
		OutputPattern: req.OutputPattern,
		PresetName:    req.PresetName,
		PresetFile:    req.PresetFile,
		DryRun:        req.DryRun,
		Quiet:         true,
	}

	result, err := torrent.ModifyTorrent(req.TorrentPath, opts)
	if err != nil {
		return nil, err
	}

	return &ModifyResult{
		OutputPath:  result.OutputPath,
		WasModified: result.WasModified,
	}, nil
}

// === Preset Operations ===

// ListPresets returns all available preset names
func (a *App) ListPresets() ([]string, error) {
	configPath, err := preset.FindPresetFile("")
	if err != nil {
		// Only ignore "not found" errors - other errors should be reported
		if err == preset.ErrPresetFileNotFound {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to locate preset file: %w", err)
	}

	config, err := preset.Load(configPath)
	if err != nil {
		return nil, err
	}

	var names []string
	for name := range config.Presets {
		names = append(names, name)
	}
	return names, nil
}

// GetPreset returns a specific preset's options
func (a *App) GetPreset(name string) (*preset.Options, error) {
	return preset.LoadPresetOptions("", name)
}

// GetPresetFilePath returns the path to the preset file
func (a *App) GetPresetFilePath() (string, error) {
	return preset.FindPresetFile("")
}

// GetAllPresets returns all presets with their full options and any loading errors
func (a *App) GetAllPresets() (*PresetsResult, error) {
	result := &PresetsResult{
		Presets: make(map[string]*preset.Options),
		Errors:  []string{},
	}

	configPath, err := preset.FindPresetFile("")
	if err != nil {
		// Only ignore "not found" errors - other errors should be reported
		if err == preset.ErrPresetFileNotFound {
			return result, nil
		}
		return nil, fmt.Errorf("failed to locate preset file: %w", err)
	}

	config, err := preset.Load(configPath)
	if err != nil {
		return nil, err
	}

	for name := range config.Presets {
		opts, err := config.GetPreset(name)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("preset %q: %v", name, err))
			continue
		}
		result.Presets[name] = opts
	}
	return result, nil
}

// validatePresetName validates a preset name for safe usage
func validatePresetName(name string) error {
	if name == "" {
		return fmt.Errorf("preset name cannot be empty")
	}
	if len(name) > 64 {
		return fmt.Errorf("preset name too long (max 64 characters)")
	}
	if strings.ContainsAny(name, "/\\:*?\"<>|") {
		return fmt.Errorf("preset name contains invalid characters")
	}
	return nil
}

// SavePreset creates or updates a preset
func (a *App) SavePreset(name string, options preset.Options) error {
	// Validate preset name
	if err := validatePresetName(name); err != nil {
		return err
	}

	// Find or create preset file path
	configPath, err := preset.FindPresetFile("")
	if err != nil {
		// Use default path if no file exists
		configPath, err = preset.GetDefaultPresetPath()
		if err != nil {
			return fmt.Errorf("could not get default preset path: %w", err)
		}
	}

	// Load or create config
	config, err := preset.LoadOrCreate(configPath)
	if err != nil {
		return fmt.Errorf("could not load preset config: %w", err)
	}

	// Update the preset
	config.Presets[name] = options

	// Save the config
	if err := preset.Save(configPath, config); err != nil {
		return fmt.Errorf("could not save preset config: %w", err)
	}

	return nil
}

// DeletePreset removes a preset from the config
func (a *App) DeletePreset(name string) error {
	configPath, err := preset.FindPresetFile("")
	if err != nil {
		return fmt.Errorf("could not find preset file: %w", err)
	}

	config, err := preset.Load(configPath)
	if err != nil {
		return fmt.Errorf("could not load preset config: %w", err)
	}

	if _, ok := config.Presets[name]; !ok {
		return fmt.Errorf("preset %q not found", name)
	}

	delete(config.Presets, name)

	// Save the config
	if err := preset.Save(configPath, config); err != nil {
		return fmt.Errorf("could not save preset config: %w", err)
	}

	return nil
}

// CreatePresetFile creates a new preset file if none exists
func (a *App) CreatePresetFile() (string, error) {
	// Check if a preset file already exists
	existingPath, err := preset.FindPresetFile("")
	if err == nil {
		return existingPath, nil
	}

	// Get default path
	configPath, err := preset.GetDefaultPresetPath()
	if err != nil {
		return "", fmt.Errorf("could not get default preset path: %w", err)
	}

	// Create empty config
	config := &preset.Config{
		Version: 1,
		Presets: make(map[string]preset.Options),
	}

	// Save the config
	if err := preset.Save(configPath, config); err != nil {
		return "", fmt.Errorf("could not create preset file: %w", err)
	}

	return configPath, nil
}

// === Tracker Operations ===

// GetTrackerInfo returns tracker-specific configuration
func (a *App) GetTrackerInfo(url string) *TrackerInfo {
	maxPieceLength, hasPieceLimit := trackers.GetTrackerMaxPieceLength(url)
	maxTorrentSize, hasTorrentLimit := trackers.GetTrackerMaxTorrentSize(url)
	defaultSource, hasSource := trackers.GetTrackerDefaultSource(url)

	return &TrackerInfo{
		MaxPieceLength: maxPieceLength,
		MaxTorrentSize: maxTorrentSize,
		DefaultSource:  defaultSource,
		HasCustomRules: hasPieceLimit || hasTorrentLimit || hasSource,
	}
}

// GetRecommendedPieceSize returns the recommended piece size for a tracker and content size
func (a *App) GetRecommendedPieceSize(trackerURL string, contentSize uint64) uint {
	exp, found := trackers.GetTrackerPieceSizeExp(trackerURL, contentSize)
	if found {
		return exp
	}
	return 0 // Let the library determine automatically
}

// GetContentSize returns the total size of the content at the given path
func (a *App) GetContentSize(path string) (uint64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}

	if !info.IsDir() {
		return uint64(info.Size()), nil
	}

	var totalSize uint64
	err = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			totalSize += uint64(info.Size())
		}
		return nil
	})

	return totalSize, err
}

// === Utility Functions ===

// applyPresetToCreateOptions applies preset options to create options
func applyPresetToCreateOptions(opts *torrent.CreateOptions, presetOpts *preset.Options) {
	if presetOpts == nil {
		return
	}

	if len(presetOpts.Trackers) > 0 && len(opts.TrackerURLs) == 0 {
		opts.TrackerURLs = presetOpts.Trackers
	}
	if len(presetOpts.WebSeeds) > 0 && len(opts.WebSeeds) == 0 {
		opts.WebSeeds = presetOpts.WebSeeds
	}
	if presetOpts.Comment != "" && opts.Comment == "" {
		opts.Comment = presetOpts.Comment
	}
	if presetOpts.Source != "" && opts.Source == "" {
		opts.Source = presetOpts.Source
	}
	if presetOpts.Private != nil {
		opts.IsPrivate = *presetOpts.Private
	}
	if presetOpts.NoDate != nil && *presetOpts.NoDate {
		opts.NoDate = true
	}
	if presetOpts.NoCreator != nil && *presetOpts.NoCreator {
		opts.NoCreator = true
	}
	if presetOpts.SkipPrefix != nil && *presetOpts.SkipPrefix {
		opts.SkipPrefix = true
	}
	if presetOpts.Entropy != nil && *presetOpts.Entropy {
		opts.Entropy = true
	}
	if presetOpts.OutputDir != "" && opts.OutputDir == "" {
		opts.OutputDir = presetOpts.OutputDir
	}
	if presetOpts.PieceLength > 0 && opts.PieceLengthExp == nil {
		pl := presetOpts.PieceLength
		opts.PieceLengthExp = &pl
	}
	if presetOpts.MaxPieceLength > 0 && opts.MaxPieceLength == nil {
		mpl := presetOpts.MaxPieceLength
		opts.MaxPieceLength = &mpl
	}
	if len(presetOpts.ExcludePatterns) > 0 && len(opts.ExcludePatterns) == 0 {
		opts.ExcludePatterns = presetOpts.ExcludePatterns
	}
	if len(presetOpts.IncludePatterns) > 0 && len(opts.IncludePatterns) == 0 {
		opts.IncludePatterns = presetOpts.IncludePatterns
	}
}

// FormatBytes formats bytes into human-readable format
func (a *App) FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// GetVersion returns the application version
func (a *App) GetVersion() string {
	return a.version
}

// OpenURL opens a URL in the default browser
func (a *App) OpenURL(rawURL string) error {
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return fmt.Errorf("invalid URL scheme: must be http or https")
	}
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}
	if parsedURL.Host == "" {
		return fmt.Errorf("invalid URL: missing host")
	}
	runtime.BrowserOpenURL(a.ctx, rawURL)
	return nil
}

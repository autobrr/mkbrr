package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/anacrolix/torrent/metainfo"
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

// CreateRequest represents a torrent creation request from the frontend
type CreateRequest struct {
	Path            string   `json:"path"`
	Name            string   `json:"name"`
	TrackerURLs     []string `json:"trackerUrls"`
	WebSeeds        []string `json:"webSeeds"`
	Comment         string   `json:"comment"`
	Source          string   `json:"source"`
	IsPrivate       bool     `json:"isPrivate"`
	PieceLengthExp  uint     `json:"pieceLengthExp"`
	MaxPieceLength  uint     `json:"maxPieceLength"`
	OutputPath      string   `json:"outputPath"`
	OutputDir       string   `json:"outputDir"`
	NoDate          bool     `json:"noDate"`
	NoCreator       bool     `json:"noCreator"`
	Entropy         bool     `json:"entropy"`
	SkipPrefix      bool     `json:"skipPrefix"`
	ExcludePatterns []string `json:"excludePatterns"`
	IncludePatterns []string `json:"includePatterns"`
	PresetName      string   `json:"presetName"`
	PresetFile      string   `json:"presetFile"`
}

// TorrentResult represents the result of torrent creation
type TorrentResult struct {
	Path       string `json:"path"`
	InfoHash   string `json:"infoHash"`
	Size       int64  `json:"size"`
	PieceCount int    `json:"pieceCount"`
	FileCount  int    `json:"fileCount"`
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

// VerifyRequest represents a verification request
type VerifyRequest struct {
	TorrentPath string `json:"torrentPath"`
	ContentPath string `json:"contentPath"`
}

// VerifyResult represents verification results
type VerifyResult struct {
	Completion   float64  `json:"completion"`
	TotalPieces  int      `json:"totalPieces"`
	GoodPieces   int      `json:"goodPieces"`
	BadPieces    int      `json:"badPieces"`
	MissingFiles []string `json:"missingFiles"`
}

// ModifyRequest represents a torrent modification request
type ModifyRequest struct {
	TorrentPath   string   `json:"torrentPath"`
	TrackerURLs   []string `json:"trackerUrls"`
	WebSeeds      []string `json:"webSeeds"`
	Comment       string   `json:"comment"`
	Source        string   `json:"source"`
	IsPrivate     *bool    `json:"isPrivate"`
	NoDate        bool     `json:"noDate"`
	NoCreator     bool     `json:"noCreator"`
	Entropy       bool     `json:"entropy"`
	SkipPrefix    bool     `json:"skipPrefix"`
	OutputDir     string   `json:"outputDir"`
	OutputPattern string   `json:"outputPattern"`
	PresetName    string   `json:"presetName"`
	PresetFile    string   `json:"presetFile"`
	DryRun        bool     `json:"dryRun"`
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

// SelectOutputDirectory opens a native directory picker for output
func (a *App) SelectOutputDirectory() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Output Directory",
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

	opts := torrent.CreateOptions{
		Path:            req.Path,
		Name:            req.Name,
		TrackerURLs:     req.TrackerURLs,
		WebSeeds:        req.WebSeeds,
		Comment:         req.Comment,
		Source:          req.Source,
		IsPrivate:       req.IsPrivate,
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
			percent := 0.0
			if total > 0 {
				percent = float64(completed) / float64(total) * 100
			}
			runtime.EventsEmit(a.ctx, "create:progress", ProgressEvent{
				Completed: completed,
				Total:     total,
				HashRate:  hashRate,
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

	t, err := torrent.LoadFromFile(info.Path)
	if err == nil {
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
		// No preset file found - return empty list, not error
		return []string{}, nil
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

// GetAllPresets returns all presets with their full options
func (a *App) GetAllPresets() (map[string]*preset.Options, error) {
	configPath, err := preset.FindPresetFile("")
	if err != nil {
		// No preset file found - return empty map
		return make(map[string]*preset.Options), nil
	}

	config, err := preset.Load(configPath)
	if err != nil {
		return nil, err
	}

	result := make(map[string]*preset.Options)
	for name := range config.Presets {
		opts, err := config.GetPreset(name)
		if err != nil {
			continue
		}
		result[name] = opts
	}
	return result, nil
}

// SavePreset creates or updates a preset
func (a *App) SavePreset(name string, options preset.Options) error {
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
func (a *App) OpenURL(url string) error {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("invalid URL scheme")
	}
	runtime.BrowserOpenURL(a.ctx, url)
	return nil
}

// Unused import prevention
var _ = metainfo.Info{}

package modify

import (
	"fmt"
	"os"
	"time"

	"github.com/anacrolix/torrent/metainfo"

	"github.com/autobrr/mkbrr/internal/preset"
	"github.com/autobrr/mkbrr/internal/torrentutils"
)

// Options represents the options for modifying a torrent,
// including both preset-related options and flag-based overrides.
type Options struct {
	PresetName     string
	PresetFile     string
	OutputDir      string
	OutputPattern  string
	NoDate         bool
	NoCreator      bool
	DryRun         bool
	Verbose        bool
	TrackerURL     string
	WebSeeds       []string
	IsPrivate      *bool
	Comment        string
	PieceLengthExp *uint
	MaxPieceLength *uint
	Source         string
	Version        string
}

// Result represents the result of modifying a torrent
type Result struct {
	Path        string
	OutputPath  string
	WasModified bool
	Error       error
}

// LoadFromFile loads a torrent file and returns a MetaInfo
func LoadFromFile(path string) (*metainfo.MetaInfo, error) {
	return torrentutils.LoadFromFile(path)
}

// ModifyTorrent modifies a single torrent file according to the given options
func ModifyTorrent(path string, opts Options) (*Result, error) {
	result := &Result{
		Path: path,
	}

	// load torrent file
	mi, err := torrentutils.LoadFromFile(path)
	if err != nil {
		result.Error = fmt.Errorf("could not load torrent: %w", err)
		return result, result.Error
	}

	// load preset if specified
	var presetOpts *preset.Options
	if opts.PresetName != "" {
		presetPath, err := preset.FindPresetFile(opts.PresetFile)
		if err != nil {
			result.Error = fmt.Errorf("could not find preset file: %w", err)
			return result, result.Error
		}

		presets, err := preset.Load(presetPath)
		if err != nil {
			result.Error = fmt.Errorf("could not load presets: %w", err)
			return result, result.Error
		}

		presetOpts, err = presets.GetPreset(opts.PresetName)
		if err != nil {
			result.Error = fmt.Errorf("could not get preset: %w", err)
			return result, result.Error
		}

		presetOpts.Version = opts.Version
	}

	// apply preset modifications if any
	wasModified := false
	if presetOpts != nil {
		wasModified, err = presetOpts.ApplyToMetaInfo(mi)
		if err != nil {
			result.Error = fmt.Errorf("could not apply preset: %w", err)
			return result, result.Error
		}
	}

	// apply flag-based overrides:
	// update tracker if flag provided
	if opts.TrackerURL != "" {
		torrentutils.UpdateTrackers(mi, opts.TrackerURL)
		wasModified = true
	}

	// update web seeds if provided via flag
	if len(opts.WebSeeds) > 0 {
		torrentutils.UpdateWebSeeds(mi, opts.WebSeeds)
		wasModified = true
	}

	// update comment if provided via flag
	if opts.Comment != "" && mi.Comment != opts.Comment {
		torrentutils.UpdateComment(mi, opts.Comment)
		wasModified = true
	}

	// update private flag if provided via flag
	if opts.IsPrivate != nil {
		modified, err := torrentutils.UpdatePrivateFlag(mi, opts.IsPrivate)
		if err == nil && modified {
			wasModified = true
		}
	}

	// update source if provided via flag
	if opts.Source != "" {
		modified, err := torrentutils.UpdateSource(mi, opts.Source)
		if err == nil && modified {
			wasModified = true
		}
	}

	// handle creator and creation date
	noCreator := opts.NoCreator
	if presetOpts != nil && presetOpts.NoCreator != nil {
		noCreator = *presetOpts.NoCreator
	}

	noDate := opts.NoDate
	if presetOpts != nil && presetOpts.NoDate != nil {
		noDate = *presetOpts.NoDate
	}

	torrentutils.UpdateCreatorAndDate(mi, fmt.Sprintf("mkbrr/%s", opts.Version), noCreator, noDate, time.Now().Unix())
	wasModified = true

	if !wasModified {
		return result, nil
	}

	if opts.DryRun {
		result.WasModified = true
		return result, nil
	}

	var metaInfoName string
	info, err := mi.UnmarshalInfo()
	if err == nil {
		metaInfoName = info.Name
	}

	// generate output path using the preset generating helper
	outPath := preset.GenerateOutputPath(path, opts.OutputDir, opts.PresetName, opts.OutputPattern, opts.TrackerURL, metaInfoName)
	result.OutputPath = outPath

	// ensure output directory exists if specified
	if opts.OutputDir != "" {
		if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
			result.Error = fmt.Errorf("could not create output directory: %w", err)
			return result, result.Error
		}
	}

	// save modified torrent file
	if err := torrentutils.SaveToFile(mi, outPath); err != nil {
		result.Error = fmt.Errorf("could not save output file: %w", err)
		return result, result.Error
	}

	result.WasModified = true
	return result, nil
}

// ProcessTorrents modifies multiple torrent files according to the given options
func ProcessTorrents(paths []string, opts Options) ([]*Result, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("no torrent files specified")
	}

	results := make([]*Result, 0, len(paths))
	for _, path := range paths {
		result, err := ModifyTorrent(path, opts)
		if err != nil {
			// continue processing other files even if one fails
			result.Error = err
		}
		results = append(results, result)
	}

	return results, nil
}

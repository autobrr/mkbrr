package torrent

import (
	"fmt"
	"os"
	"time"

	"github.com/anacrolix/torrent/bencode"
	"github.com/anacrolix/torrent/metainfo"

	"github.com/autobrr/mkbrr/internal/preset"
)

// ModifyOptions represents the options for modifying a torrent,
// including both preset-related options and flag-based overrides.
type ModifyOptions struct {
	IsPrivate      *bool
	PieceLengthExp *uint
	MaxPieceLength *uint
	PresetName     string
	PresetFile     string
	Name           string
	OutputDir      string
	OutputPattern  string
	TrackerURLs    []string
	Comment        string
	Source         string
	Version        string
	WebSeeds       []string
	NoDate         bool
	NoCreator      bool
	DryRun         bool
	Verbose        bool
	Quiet          bool
	Entropy        *bool
	SkipPrefix     bool
	SourceSet      bool // true when --source flag was explicitly provided (allows empty string to clear)
	CommentSet     bool // true when --comment flag was explicitly provided (allows empty string to clear)
	RemovePrivate  bool // true when --no-private flag is provided (removes private field entirely)
}

// Result represents the result of modifying a torrent
type Result struct {
	Error       error
	Path        string
	OutputPath  string
	WasModified bool
}

// LoadFromFile loads a torrent file from disk and returns a Torrent struct.
// The returned Torrent wraps the metainfo and provides additional functionality.
func LoadFromFile(path string) (*Torrent, error) {
	mi, err := metainfo.LoadFromFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not load torrent: %w", err)
	}
	return &Torrent{MetaInfo: mi}, nil
}

// ModifyTorrent modifies a single torrent file according to the given options.
// It can change trackers, comment, source, piece length, and other metadata.
// Returns a Result containing the operation outcome and output path.
func ModifyTorrent(path string, opts ModifyOptions) (*Result, error) {
	result := &Result{
		Path: path,
	}

	// load torrent file
	mi, err := metainfo.LoadFromFile(path)
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

	// read current info values via struct (for comparisons only — never marshal this back)
	info, err := mi.UnmarshalInfo()
	if err != nil {
		result.Error = fmt.Errorf("could not unmarshal info: %w", err)
		return result, result.Error
	}
	originalMetaInfoName := info.Name

	// track info-level changes to apply via raw map at the end,
	// preserving any custom keys (e.g. entropy) that the typed struct would drop
	type infoChange struct {
		key    string
		value  any
		remove bool
	}
	var infoChanges []infoChange

	// apply flag-based overrides:
	// update tracker if flag provided
	if len(opts.TrackerURLs) > 0 {
		mi.Announce = opts.TrackerURLs[0] // Primary announce is the first one
		announceList := make([][]string, len(opts.TrackerURLs))
		for i, tracker := range opts.TrackerURLs {
			announceList[i] = []string{tracker}
		}
		mi.AnnounceList = announceList
		wasModified = true
		// Note: This overrides any trackers set by a preset
	}

	// update name if provided via flag
	if opts.Name != "" && info.Name != opts.Name {
		infoChanges = append(infoChanges, infoChange{key: "name", value: opts.Name})
		wasModified = true
	}

	// update web seeds if provided via flag
	if len(opts.WebSeeds) > 0 {
		mi.UrlList = opts.WebSeeds
		wasModified = true
	}

	// update comment if provided via flag (CommentSet allows clearing with empty string)
	if opts.CommentSet {
		if opts.Comment == "" || mi.Comment != opts.Comment {
			mi.Comment = opts.Comment
			wasModified = true
		}
	} else if opts.Comment != "" && mi.Comment != opts.Comment {
		mi.Comment = opts.Comment
		wasModified = true
	}

	// remove private field entirely if requested
	if opts.RemovePrivate {
		infoChanges = append(infoChanges, infoChange{key: "private", remove: true})
		wasModified = true
	} else if opts.IsPrivate != nil {
		if info.Private == nil || *info.Private != *opts.IsPrivate {
			val := int64(0)
			if *opts.IsPrivate {
				val = 1
			}
			infoChanges = append(infoChanges, infoChange{key: "private", value: val})
			wasModified = true
		}
	}

	// update source if provided via flag (SourceSet allows clearing with empty string)
	if opts.SourceSet {
		if opts.Source == "" {
			// explicitly remove the source key from info dict
			infoChanges = append(infoChanges, infoChange{key: "source", remove: true})
			wasModified = true
		} else if info.Source != opts.Source {
			infoChanges = append(infoChanges, infoChange{key: "source", value: opts.Source})
			wasModified = true
		}
	} else if opts.Source != "" && info.Source != opts.Source {
		infoChanges = append(infoChanges, infoChange{key: "source", value: opts.Source})
		wasModified = true
	}

	// apply entropy from preset if not explicitly set via flag
	if opts.Entropy == nil && presetOpts != nil && presetOpts.Entropy != nil {
		opts.Entropy = presetOpts.Entropy
	}

	// add random entropy field for cross-seeding if enabled
	if opts.Entropy != nil && *opts.Entropy {
		if entropy, err := generateRandomString(); err == nil {
			infoChanges = append(infoChanges, infoChange{key: "entropy", value: entropy})
			wasModified = true
		}
	}

	// apply all info-level changes via raw map to preserve custom keys
	if len(infoChanges) > 0 {
		infoMap := make(map[string]any)
		if err := bencode.Unmarshal(mi.InfoBytes, &infoMap); err != nil {
			result.Error = fmt.Errorf("could not unmarshal info map: %w", err)
			return result, result.Error
		}
		for _, c := range infoChanges {
			if c.remove {
				delete(infoMap, c.key)
			} else {
				infoMap[c.key] = c.value
			}
		}
		infoBytes, err := bencode.Marshal(infoMap)
		if err != nil {
			result.Error = fmt.Errorf("could not marshal info map: %w", err)
			return result, result.Error
		}
		mi.InfoBytes = infoBytes
	}

	// handle creator
	if presetOpts != nil && presetOpts.NoCreator != nil && *presetOpts.NoCreator || opts.NoCreator {
		mi.CreatedBy = ""
		wasModified = true
	}

	// update creation date based on preset and command line options
	if presetOpts != nil && presetOpts.NoDate != nil && *presetOpts.NoDate || opts.NoDate {
		mi.CreationDate = 0
		wasModified = true
	} else {
		mi.CreationDate = time.Now().Unix()
		wasModified = true
	}

	if !wasModified {
		return result, nil
	}

	if opts.DryRun {
		result.WasModified = true
		return result, nil
	}

	// re-read info to get potentially updated name (e.g. if name was changed via infoChanges)
	var metaInfoName string
	if updatedInfo, infoErr := mi.UnmarshalInfo(); infoErr == nil {
		metaInfoName = updatedInfo.Name
	}

	basePath := path
	if opts.OutputPattern == "" && originalMetaInfoName != "" {
		basePath = originalMetaInfoName + ".torrent"
	}

	// determine output directory: command-line flag takes precedence over preset
	outputDir := opts.OutputDir
	if outputDir == "" && presetOpts != nil && presetOpts.OutputDir != "" {
		outputDir = presetOpts.OutputDir
	}

	// generate output path using the preset generating helper
	var trackerForOutput string
	if len(opts.TrackerURLs) > 0 {
		trackerForOutput = opts.TrackerURLs[0]
	} else {
		trackerForOutput = ""
	}
	outPath := preset.GenerateOutputPath(basePath, outputDir, opts.PresetName, opts.OutputPattern, trackerForOutput, metaInfoName, opts.SkipPrefix)
	result.OutputPath = outPath

	// ensure output directory exists if specified
	if outputDir != "" {
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			result.Error = fmt.Errorf("could not create output directory: %w", err)
			return result, result.Error
		}
	}

	// save modified torrent file
	f, err := os.Create(outPath)
	if err != nil {
		result.Error = fmt.Errorf("could not create output file: %w", err)
		return result, result.Error
	}
	defer f.Close()

	if err := mi.Write(f); err != nil {
		result.Error = fmt.Errorf("could not write output file: %w", err)
		return result, result.Error
	}

	result.WasModified = true
	return result, nil
}

// ProcessTorrents modifies multiple torrent files according to the given options.
// It processes each torrent file and returns the results for all operations.
// This function provides parallel processing for better performance with multiple files.
func ProcessTorrents(paths []string, opts ModifyOptions) ([]*Result, error) {
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

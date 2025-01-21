package torrent

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// PresetConfig represents the YAML configuration for torrent creation presets
type PresetConfig struct {
	Version int                   `yaml:"version"`
	Default *PresetOpts           `yaml:"default"`
	Presets map[string]PresetOpts `yaml:"presets"`
}

// PresetOpts represents the options for a single preset
type PresetOpts struct {
	Trackers    []string `yaml:"trackers"`
	WebSeeds    []string `yaml:"webseeds"`
	Private     bool     `yaml:"private"`
	PieceLength uint     `yaml:"piece_length"`
	Comment     string   `yaml:"comment"`
	Source      string   `yaml:"source"`
	NoDate      bool     `yaml:"no_date"`
}

// LoadPresets loads presets from a config file
func LoadPresets(configPath string) (*PresetConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read preset config: %w", err)
	}

	var config PresetConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse preset config: %w", err)
	}

	if config.Version != 1 {
		return nil, fmt.Errorf("unsupported preset config version: %d", config.Version)
	}

	if len(config.Presets) == 0 {
		return nil, fmt.Errorf("no presets defined in config")
	}

	return &config, nil
}

// GetPreset returns a preset by name, merged with default settings
func (c *PresetConfig) GetPreset(name string) (*PresetOpts, error) {
	preset, ok := c.Presets[name]
	if !ok {
		return nil, fmt.Errorf("preset %q not found", name)
	}

	// if we have defaults, merge them with the preset
	if c.Default != nil {
		merged := *c.Default // create a copy of defaults

		// override defaults with preset-specific values
		if len(preset.Trackers) > 0 {
			merged.Trackers = preset.Trackers
		}
		if len(preset.WebSeeds) > 0 {
			merged.WebSeeds = preset.WebSeeds
		}
		if preset.PieceLength != 0 {
			merged.PieceLength = preset.PieceLength
		}
		if preset.Comment != "" {
			merged.Comment = preset.Comment
		}
		if preset.Source != "" {
			merged.Source = preset.Source
		}

		// explicit bool overrides
		if preset.Private != merged.Private {
			merged.Private = preset.Private
		}
		if preset.NoDate != merged.NoDate {
			merged.NoDate = preset.NoDate
		}

		return &merged, nil
	}

	// if no defaults, just return the preset
	return &preset, nil
}

// ToCreateOptions converts a PresetOpts to CreateTorrentOptions
func (p *PresetOpts) ToCreateOptions(path string, verbose bool, version string) CreateTorrentOptions {
	var tracker string
	if len(p.Trackers) > 0 {
		tracker = p.Trackers[0]
	}

	opts := CreateTorrentOptions{
		Path:       path,
		TrackerURL: tracker,
		WebSeeds:   p.WebSeeds,
		IsPrivate:  p.Private,
		Comment:    p.Comment,
		Source:     p.Source,
		NoDate:     p.NoDate,
		Verbose:    verbose,
		Version:    version,
	}

	if p.PieceLength != 0 {
		pieceLen := p.PieceLength
		opts.PieceLengthExp = &pieceLen
	}

	return opts
}

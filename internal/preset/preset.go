package preset

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/anacrolix/torrent/bencode"
	"github.com/anacrolix/torrent/metainfo"
	"gopkg.in/yaml.v3"
)

// Config represents the YAML configuration for torrent creation presets
type Config struct {
	Version int                `yaml:"version"`
	Default *Options           `yaml:"default"`
	Presets map[string]Options `yaml:"presets"`
}

// Options represents the options for a single preset
type Options struct {
	Trackers       []string `yaml:"trackers"`
	WebSeeds       []string `yaml:"webseeds"`
	Private        bool     `yaml:"private"`
	PieceLength    uint     `yaml:"piece_length"`
	MaxPieceLength uint     `yaml:"max_piece_length"`
	Comment        string   `yaml:"comment"`
	Source         string   `yaml:"source"`
	NoDate         bool     `yaml:"no_date"`
}

// FindPresetFile searches for a preset file in known locations
func FindPresetFile(explicitPath string) (string, error) {
	// check known locations in order
	locations := []string{
		explicitPath,   // explicitly specified file
		"presets.yaml", // current directory
	}

	// add user home directory locations
	if home, err := os.UserHomeDir(); err == nil {
		locations = append(locations,
			filepath.Join(home, ".config", "mkbrr", "presets.yaml"), // ~/.config/mkbrr/
			filepath.Join(home, ".mkbrr", "presets.yaml"),           // ~/.mkbrr/
		)
	}

	// find first existing preset file
	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			return loc, nil
		}
	}

	return "", fmt.Errorf("could not find preset file in known locations")
}

// Load loads presets from a config file
func Load(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("could not read preset config: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("could not parse preset config: %w", err)
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
func (c *Config) GetPreset(name string) (*Options, error) {
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
		if preset.MaxPieceLength != 0 {
			merged.MaxPieceLength = preset.MaxPieceLength
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

// ApplyToMetaInfo applies preset options to a MetaInfo object
func (o *Options) ApplyToMetaInfo(mi *metainfo.MetaInfo) (bool, error) {
	wasModified := false

	info, err := mi.UnmarshalInfo()
	if err != nil {
		return false, fmt.Errorf("could not unmarshal info: %w", err)
	}

	// apply tracker URL
	if len(o.Trackers) > 0 && (len(mi.AnnounceList) == 0 || mi.AnnounceList[0][0] != o.Trackers[0]) {
		mi.Announce = o.Trackers[0]
		mi.AnnounceList = [][]string{{o.Trackers[0]}}
		wasModified = true
	}

	// apply source tag
	if o.Source != "" && info.Source != o.Source {
		info.Source = o.Source
		wasModified = true
		// re-marshal the modified info
		if infoBytes, err := bencode.Marshal(info); err == nil {
			mi.InfoBytes = infoBytes
		}
	}

	// apply private flag
	isPrivate := info.Private != nil && *info.Private
	if isPrivate != o.Private {
		info.Private = &o.Private
		wasModified = true
		// re-marshal the modified info
		if infoBytes, err := bencode.Marshal(info); err == nil {
			mi.InfoBytes = infoBytes
		}
	}

	return wasModified, nil
}

// GenerateOutputPath generates an output path for a modified torrent file
func GenerateOutputPath(originalPath, outputDir, presetName string) string {
	dir := filepath.Dir(originalPath)
	if outputDir != "" {
		dir = outputDir
	}

	base := filepath.Base(originalPath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	// add preset name or "modified" to filename
	suffix := "-modified"
	if presetName != "" {
		suffix = "-" + presetName
	}

	return filepath.Join(dir, name+suffix+ext)
}

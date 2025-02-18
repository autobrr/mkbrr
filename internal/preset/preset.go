package preset

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	Private        *bool    `yaml:"private"`
	PieceLength    uint     `yaml:"piece_length"`
	MaxPieceLength uint     `yaml:"max_piece_length"`
	PiecesTarget   uint     `yaml:"pieces_target"`
	Comment        string   `yaml:"comment"`
	Source         string   `yaml:"source"`
	NoDate         *bool    `yaml:"no_date"`
	NoCreator      *bool    `yaml:"no_creator"`
	Version        string   // used for creator string
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

	// create a copy with hardcoded defaults
	defaultPrivate := true
	defaultNoDate := false
	defaultNoCreator := false

	merged := Options{
		Private:   &defaultPrivate,
		NoDate:    &defaultNoDate,
		NoCreator: &defaultNoCreator,
	}

	// if we have defaults in config, use those instead
	if c.Default != nil {
		if c.Default.Private != nil {
			merged.Private = c.Default.Private
		}
		if c.Default.NoDate != nil {
			merged.NoDate = c.Default.NoDate
		}
		if c.Default.NoCreator != nil {
			merged.NoCreator = c.Default.NoCreator
		}
		merged.Trackers = c.Default.Trackers
		merged.WebSeeds = c.Default.WebSeeds
		merged.Comment = c.Default.Comment
		merged.Source = c.Default.Source
		merged.PieceLength = c.Default.PieceLength
		merged.MaxPieceLength = c.Default.MaxPieceLength
	}

	// override with preset values if they are set
	if len(preset.Trackers) > 0 {
		merged.Trackers = preset.Trackers
	}
	if len(preset.WebSeeds) > 0 {
		merged.WebSeeds = preset.WebSeeds
	}
	if preset.Comment != "" {
		merged.Comment = preset.Comment
	}
	if preset.Source != "" {
		merged.Source = preset.Source
	}
	if preset.PieceLength != 0 {
		merged.PieceLength = preset.PieceLength
	}
	if preset.MaxPieceLength != 0 {
		merged.MaxPieceLength = preset.MaxPieceLength
	}
	if preset.Private != nil {
		merged.Private = preset.Private
	}
	if preset.NoDate != nil {
		merged.NoDate = preset.NoDate
	}
	if preset.NoCreator != nil {
		merged.NoCreator = preset.NoCreator
	}

	return &merged, nil
}

// ApplyToMetaInfo applies preset options to a MetaInfo object
func (o *Options) ApplyToMetaInfo(mi *metainfo.MetaInfo) (bool, error) {
	wasModified := false

	info, err := mi.UnmarshalInfo()
	if err != nil {
		return false, fmt.Errorf("could not unmarshal info: %w", err)
	}

	// Only modify values that are explicitly set in the preset
	if len(o.Trackers) > 0 {
		mi.Announce = o.Trackers[0]
		mi.AnnounceList = [][]string{o.Trackers}
		wasModified = true
	}

	if len(o.WebSeeds) > 0 {
		mi.UrlList = o.WebSeeds
		wasModified = true
	}

	if o.Source != "" {
		info.Source = o.Source
		wasModified = true
	}

	if o.Comment != "" {
		mi.Comment = o.Comment
		wasModified = true
	}

	if o.Private != nil {
		if info.Private == nil {
			info.Private = new(bool)
		}
		*info.Private = *o.Private
		wasModified = true
	}

	if o.NoCreator != nil {
		if *o.NoCreator {
			mi.CreatedBy = ""
		} else {
			mi.CreatedBy = fmt.Sprintf("mkbrr/%s", o.Version)
		}
		wasModified = true
	}

	if o.NoDate != nil {
		if *o.NoDate {
			mi.CreationDate = 0
		} else {
			mi.CreationDate = time.Now().Unix()
		}
		wasModified = true
	}

	// re-marshal the modified info if needed
	if wasModified {
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

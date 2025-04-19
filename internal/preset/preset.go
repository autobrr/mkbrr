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
	Default *Options           `yaml:"default"`
	Presets map[string]Options `yaml:"presets"`
	Version int                `yaml:"version"`
}

// Options represents the options for a single preset
type Options struct {
	Private         *bool    `yaml:"private"`
	NoDate          *bool    `yaml:"no_date"`
	NoCreator       *bool    `yaml:"no_creator"`
	SkipPrefix      *bool    `yaml:"skip_prefix"`
	Entropy         *bool    `yaml:"entropy"`
	Comment         string   `yaml:"comment"`
	Source          string   `yaml:"source"`
	Version         string   // used for creator string
	Trackers        []string `yaml:"trackers"`
	WebSeeds        []string `yaml:"webseeds"`
	ExcludePatterns []string `yaml:"exclude_patterns"`
	IncludePatterns []string `yaml:"include_patterns"`
	PieceLength     uint     `yaml:"piece_length"`
	MaxPieceLength  uint     `yaml:"max_piece_length"`
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
	defaultSkipPrefix := false

	merged := Options{
		Private:    &defaultPrivate,
		NoDate:     &defaultNoDate,
		NoCreator:  &defaultNoCreator,
		SkipPrefix: &defaultSkipPrefix,
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
		if c.Default.SkipPrefix != nil {
			merged.SkipPrefix = c.Default.SkipPrefix
		}
		merged.Trackers = c.Default.Trackers
		merged.WebSeeds = c.Default.WebSeeds
		merged.Comment = c.Default.Comment
		merged.Source = c.Default.Source
		merged.PieceLength = c.Default.PieceLength
		merged.MaxPieceLength = c.Default.MaxPieceLength
		if len(c.Default.ExcludePatterns) > 0 {
			merged.ExcludePatterns = c.Default.ExcludePatterns
		}
		if len(c.Default.IncludePatterns) > 0 {
			merged.IncludePatterns = c.Default.IncludePatterns
		}
		if c.Default.Entropy != nil {
			merged.Entropy = c.Default.Entropy
		}
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
	if preset.SkipPrefix != nil {
		merged.SkipPrefix = preset.SkipPrefix
	}
	if len(preset.ExcludePatterns) > 0 {
		merged.ExcludePatterns = preset.ExcludePatterns
	}
	if len(preset.IncludePatterns) > 0 {
		merged.IncludePatterns = preset.IncludePatterns
	}
	if preset.Entropy != nil {
		merged.Entropy = preset.Entropy
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

// GetDomainPrefix extracts a clean domain name from a tracker URL to use as a filename prefix
func GetDomainPrefix(trackerURL string) string {
	if trackerURL == "" {
		return "modified"
	}

	cleanURL := strings.TrimSpace(trackerURL)

	domain := cleanURL

	if strings.Contains(domain, "://") {
		parts := strings.SplitN(domain, "://", 2)
		if len(parts) == 2 {
			domain = parts[1]
		}
	}

	if strings.Contains(domain, "/") {
		domain = strings.SplitN(domain, "/", 2)[0]
	}

	if strings.Contains(domain, ":") {
		domain = strings.SplitN(domain, ":", 2)[0]
	}

	domain = strings.TrimPrefix(domain, "www.")

	if domain != "" {
		parts := strings.Split(domain, ".")

		if len(parts) > 1 {
			// take only the domain name without TLD
			// for example, from "tracker.example.com", get "example"
			if len(parts) > 2 {
				// for subdomains, use the second-to-last part
				domain = parts[len(parts)-2]
			} else {
				// for simple domains like example.com, use the first part
				domain = parts[0]
			}
		}

		return sanitizeFilename(domain)
	}

	return "modified"
}

// GenerateOutputPath generates an output path for a modified torrent file
func GenerateOutputPath(originalPath, outputDir, presetName string, outputPattern string, trackerURL string, metaInfoName string, skipPrefix bool) string {
	dir := filepath.Dir(originalPath)
	if outputDir != "" {
		dir = outputDir
	}

	base := filepath.Base(originalPath)
	ext := filepath.Ext(base)

	name := strings.TrimSuffix(base, ext)

	// if custom output pattern is provided, use it
	if outputPattern != "" {
		return filepath.Join(dir, outputPattern+ext)
	}

	// if skip-prefix is true, just return the original filename
	if skipPrefix {
		return filepath.Join(dir, base)
	}

	// prioritize preset name over tracker URL
	var prefix string
	if presetName != "" {
		prefix = sanitizeFilename(presetName)
	} else {
		prefix = GetDomainPrefix(trackerURL)
	}

	return filepath.Join(dir, prefix+"_"+name+ext)
}

// sanitizeFilename removes characters that are invalid in filenames
func sanitizeFilename(input string) string {
	// replace characters that are problematic in filenames
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
		" ", "_",
	)
	return replacer.Replace(input)
}

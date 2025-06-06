package torrent

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/autobrr/mkbrr/internal/preset"
)

// BatchConfig represents the YAML configuration for batch torrent creation
type BatchConfig struct {
	Jobs    []BatchJob `yaml:"jobs"`
	Version int        `yaml:"version"`
}

// BatchJob represents a single torrent creation job within a batch
type BatchJob struct {
	Output              string   `yaml:"output"`
	Path                string   `yaml:"path"`
	Name                string   `yaml:"-"`
	Comment             string   `yaml:"comment"`
	Source              string   `yaml:"source"`
	Trackers            []string `yaml:"trackers"`
	WebSeeds            []string `yaml:"webseeds"`
	ExcludePatterns     []string `yaml:"exclude_patterns"`
	IncludePatterns     []string `yaml:"include_patterns"`
	PieceLength         uint     `yaml:"piece_length"`
	Private             bool     `yaml:"private"`
	NoDate              bool     `yaml:"no_date"`
	SkipPrefix          bool     `yaml:"skip_prefix"`
	FailOnSeasonWarning bool     `yaml:"fail_on_season_warning"`
}

// ToCreateOptions converts a BatchJob to CreateTorrentOptions
func (j *BatchJob) ToCreateOptions(verbose bool, quiet bool, version string) CreateTorrentOptions {
	var tracker string
	if len(j.Trackers) > 0 {
		tracker = j.Trackers[0]
	}

	opts := CreateTorrentOptions{
		Path:                    j.Path,
		Name:                    j.Name,
		TrackerURL:              tracker,
		WebSeeds:                j.WebSeeds,
		IsPrivate:               j.Private,
		Comment:                 j.Comment,
		Source:                  j.Source,
		NoDate:                  j.NoDate,
		Verbose:                 verbose,
		Quiet:                   quiet,
		Version:                 version,
		SkipPrefix:              j.SkipPrefix,
		ExcludePatterns:         j.ExcludePatterns,
		IncludePatterns:         j.IncludePatterns,
		FailOnSeasonPackWarning: j.FailOnSeasonWarning,
	}

	if j.PieceLength != 0 {
		pieceLen := j.PieceLength
		opts.PieceLengthExp = &pieceLen
	}

	return opts
}

// BatchResult represents the result of a single job in the batch
type BatchResult struct {
	Error    error
	Info     *TorrentInfo
	Trackers []string
	Job      BatchJob
	Success  bool
}

// ProcessBatch processes a batch configuration file and creates multiple torrents
func ProcessBatch(configPath string, verbose bool, quiet bool, version string) ([]BatchResult, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read batch config: %w", err)
	}

	var config BatchConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse batch config: %w", err)
	}

	if config.Version != 1 {
		return nil, fmt.Errorf("unsupported batch config version: %d", config.Version)
	}

	if len(config.Jobs) == 0 {
		return nil, fmt.Errorf("no jobs defined in batch config")
	}

	// validate all jobs before processing
	for _, job := range config.Jobs {
		if err := validateJob(job); err != nil {
			return nil, fmt.Errorf("invalid job configuration: %w", err)
		}
	}

	results := make([]BatchResult, len(config.Jobs))
	var wg sync.WaitGroup

	// process jobs in parallel with a worker pool
	workers := minInt(len(config.Jobs), 4) // limit concurrent jobs
	jobs := make(chan int, len(config.Jobs))

	// start workers
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				results[idx] = processJob(config.Jobs[idx], verbose, quiet, version)
			}
		}()
	}

	// send jobs to workers
	for i := range config.Jobs {
		jobs <- i
	}
	close(jobs)

	wg.Wait()
	return results, nil
}

func validateJob(job BatchJob) error {
	if job.Path == "" {
		return fmt.Errorf("path is required")
	}

	if _, err := os.Stat(job.Path); err != nil {
		return fmt.Errorf("invalid path %q: %w", job.Path, err)
	}

	if job.Output == "" {
		return fmt.Errorf("output is required")
	}

	if job.PieceLength != 0 && (job.PieceLength < 14 || job.PieceLength > 24) {
		return fmt.Errorf("piece length must be between 14 and 24")
	}

	return nil
}

func processJob(job BatchJob, verbose bool, quiet bool, version string) BatchResult {
	result := BatchResult{
		Job:      job,
		Trackers: job.Trackers,
	}

	var trackerURL string
	if len(job.Trackers) > 0 {
		trackerURL = job.Trackers[0]
	}

	output := job.Output
	if output == "" {
		baseName := filepath.Base(filepath.Clean(job.Path))

		if trackerURL != "" && !job.SkipPrefix {
			prefix := preset.GetDomainPrefix(trackerURL)
			baseName = prefix + "_" + baseName
		}

		output = baseName
	}

	// ensure output has .torrent extension
	if filepath.Ext(output) != ".torrent" {
		output += ".torrent"
	}

	// convert job to CreateTorrentOptions
	opts := job.ToCreateOptions(verbose, quiet, version)

	// create the torrent
	mi, err := CreateTorrent(opts)
	if err != nil {
		result.Error = fmt.Errorf("failed to create torrent: %w", err)
		return result
	}

	// write the torrent file
	f, err := os.Create(output)
	if err != nil {
		result.Error = fmt.Errorf("failed to create output file: %w", err)
		return result
	}
	defer f.Close()

	if err := mi.Write(f); err != nil {
		result.Error = fmt.Errorf("failed to write torrent file: %w", err)
		return result
	}

	// collect torrent info
	info := mi.GetInfo()
	result.Success = true
	result.Info = &TorrentInfo{
		Path:     output,
		Size:     info.TotalLength(),
		InfoHash: mi.HashInfoBytes().String(),
		Files:    len(info.Files),
	}

	return result
}

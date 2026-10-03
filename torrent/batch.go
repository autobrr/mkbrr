// Copyright (c) 2025-2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

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
	Private             *bool    `yaml:"private"`
	Source              *string  `yaml:"source"`
	Output              string   `yaml:"output"`
	Path                string   `yaml:"path"`
	Name                string   `yaml:"-"`
	Comment             string   `yaml:"comment"`
	Trackers            []string `yaml:"trackers"`
	WebSeeds            []string `yaml:"webseeds"`
	ExcludePatterns     []string `yaml:"exclude_patterns"`
	IncludePatterns     []string `yaml:"include_patterns"`
	PieceLength         uint     `yaml:"piece_length"`
	MaxPieceLength      uint     `yaml:"max_piece_length"`
	TargetPieceCount    uint     `yaml:"target_piece_count"`
	NoDate              bool     `yaml:"no_date"`
	NoCreator           bool     `yaml:"no_creator"`
	SkipPrefix          bool     `yaml:"skip_prefix"`
	Entropy             bool     `yaml:"entropy"`
	FailOnSeasonWarning bool     `yaml:"fail_on_season_warning"`
}

// ToCreateOptions resolves a BatchJob into CreateOptions. Batch jobs have no preset.
func (j *BatchJob) ToCreateOptions(verbose bool, quiet bool, infoOnly bool, version string) (CreateOptions, error) {
	return ResolveCreateOptions(CreateOverrides{
		Path:                    j.Path,
		Name:                    j.Name,
		Version:                 version,
		Verbose:                 verbose,
		Quiet:                   quiet,
		InfoOnly:                infoOnly,
		IsPrivate:               j.Private,
		TrackerURLs:             j.Trackers,
		WebSeeds:                j.WebSeeds,
		ExcludePatterns:         j.ExcludePatterns,
		IncludePatterns:         j.IncludePatterns,
		Comment:                 nonZero(j.Comment),
		Source:                  j.Source,
		PieceLengthExp:          nonZero(j.PieceLength),
		MaxPieceLength:          nonZero(j.MaxPieceLength),
		TargetPieceCount:        nonZero(j.TargetPieceCount),
		NoDate:                  &j.NoDate,
		NoCreator:               &j.NoCreator,
		SkipPrefix:              &j.SkipPrefix,
		Entropy:                 &j.Entropy,
		FailOnSeasonPackWarning: &j.FailOnSeasonWarning,
	}, nil)
}

// BatchResult represents the result of a single job in the batch
type BatchResult struct {
	Error    error
	Info     *TorrentInfo
	Trackers []string
	Job      BatchJob
	Success  bool
}

// ProcessBatch processes a batch configuration file and creates multiple torrents.
// It reads a YAML configuration file containing multiple torrent creation jobs
// and processes them in parallel for efficient batch operations.
func ProcessBatch(configPath string, verbose bool, quiet bool, infoOnly bool, version string) ([]BatchResult, error) {
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
	workers := min(len(config.Jobs), 4) // limit concurrent jobs
	jobs := make(chan int, len(config.Jobs))

	// start workers
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				results[idx] = processJob(config.Jobs[idx], verbose, quiet, infoOnly, version)
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

	if job.PieceLength != 0 && job.TargetPieceCount != 0 {
		return fmt.Errorf("cannot set both piece_length and target_piece_count; use one or the other")
	}

	return nil
}

func processJob(job BatchJob, verbose bool, quiet bool, infoOnly bool, version string) BatchResult {
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

	// convert job to CreateOptions
	opts, err := job.ToCreateOptions(verbose, quiet, infoOnly, version)
	if err != nil {
		result.Error = fmt.Errorf("invalid job options: %w", err)
		return result
	}

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

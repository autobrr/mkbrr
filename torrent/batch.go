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
	CreateSettings `yaml:",inline"`
	Output         string `yaml:"output"`
	Path           string `yaml:"path"`
	Name           string `yaml:"-"`
}

// normalizedSettings returns a copy of the job's create settings. A zero piece setting
// means "not set" in a batch job, so the copy has nil in its place.
func (j *BatchJob) normalizedSettings() CreateSettings {
	s := j.CreateSettings
	s.PieceLengthExp = nilIfZero(s.PieceLengthExp)
	s.MaxPieceLength = nilIfZero(s.MaxPieceLength)
	s.TargetPieceCount = nilIfZero(s.TargetPieceCount)
	return s
}

// ToCreateOptions resolves a BatchJob into CreateOptions. Batch jobs have no preset.
func (j *BatchJob) ToCreateOptions(verbose bool, quiet bool, infoOnly bool, version string) (CreateOptions, error) {
	return ResolveCreateOptions(CreateOverrides{
		CreateSettings: j.normalizedSettings(),
		Path:           j.Path,
		Name:           j.Name,
		Version:        version,
		Verbose:        verbose,
		Quiet:          quiet,
		InfoOnly:       infoOnly,
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

func nilIfZero(p *uint) *uint {
	if p == nil || *p == 0 {
		return nil
	}
	return p
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

	s := job.normalizedSettings()
	// create applies the stricter bounds for each tracker
	if exp := s.PieceLengthExp; exp != nil && (*exp < 14 || *exp > 27) {
		return fmt.Errorf("piece length must be between 14 and 27")
	}

	if s.PieceLengthExp != nil && s.TargetPieceCount != nil {
		return fmt.Errorf("cannot set both piece_length and target_piece_count; use one or the other")
	}

	return nil
}

func processJob(job BatchJob, verbose bool, quiet bool, infoOnly bool, version string) BatchResult {
	result := BatchResult{
		Job:      job,
		Trackers: job.TrackerURLs,
	}

	// convert job to CreateOptions
	opts, err := job.ToCreateOptions(verbose, quiet, infoOnly, version)
	if err != nil {
		result.Error = fmt.Errorf("invalid job options: %w", err)
		return result
	}

	var trackerURL string
	if len(opts.TrackerURLs) > 0 {
		trackerURL = opts.TrackerURLs[0]
	}

	output := job.Output
	if output == "" {
		baseName := filepath.Base(filepath.Clean(job.Path))

		if trackerURL != "" && !opts.SkipPrefix {
			prefix := preset.GetDomainPrefix(trackerURL)
			baseName = prefix + "_" + baseName
		}

		output = baseName
	}

	// ensure output has .torrent extension
	if filepath.Ext(output) != ".torrent" {
		output += ".torrent"
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

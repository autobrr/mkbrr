package torrent

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

// BatchConfig represents the YAML configuration for batch torrent creation
type BatchConfig struct {
	Version int        `yaml:"version"`
	Jobs    []BatchJob `yaml:"jobs"`
}

// BatchJob represents a single torrent creation job within a batch
type BatchJob struct {
	Output      string   `yaml:"output"`
	Path        string   `yaml:"path"`
	Name        string   `yaml:"name"`
	Trackers    []string `yaml:"trackers"`
	WebSeeds    []string `yaml:"webseeds"`
	Private     bool     `yaml:"private"`
	PieceLength uint     `yaml:"piece_length"`
	Comment     string   `yaml:"comment"`
	Source      string   `yaml:"source"`
	NoDate      bool     `yaml:"no_date"`
}

// ToCreateOptions converts a BatchJob to CreateTorrentOptions
func (j *BatchJob) ToCreateOptions(verbose bool, version string) CreateTorrentOptions {
	var tracker string
	if len(j.Trackers) > 0 {
		tracker = j.Trackers[0]
	}

	opts := CreateTorrentOptions{
		Path:       j.Path,
		Name:       j.Name,
		TrackerURL: tracker,
		WebSeeds:   j.WebSeeds,
		IsPrivate:  j.Private,
		Comment:    j.Comment,
		Source:     j.Source,
		NoDate:     j.NoDate,
		Verbose:    verbose,
		Version:    version,
	}

	if j.PieceLength != 0 {
		pieceLen := j.PieceLength
		opts.PieceLengthExp = &pieceLen
	}

	return opts
}

// BatchResult represents the result of a single job in the batch
type BatchResult struct {
	Job      BatchJob
	Success  bool
	Error    error
	Info     *TorrentInfo
	Trackers []string
}

// TorrentInfo contains summary information about the created torrent
type TorrentInfo struct {
	Path     string
	Size     int64
	InfoHash string
	Files    int
	Announce string
}

// ProcessBatch processes a batch configuration file and creates multiple torrents
func ProcessBatch(configPath string, verbose bool, version string) ([]BatchResult, error) {
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

	// Validate all jobs before processing
	for _, job := range config.Jobs {
		if err := validateJob(job); err != nil {
			return nil, fmt.Errorf("invalid job configuration: %w", err)
		}
	}

	results := make([]BatchResult, len(config.Jobs))
	var wg sync.WaitGroup

	// Process jobs in parallel with a worker pool
	workers := minInt(len(config.Jobs), 4) // Limit concurrent jobs
	jobs := make(chan int, len(config.Jobs))

	// Start workers
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				results[idx] = processJob(config.Jobs[idx], verbose, version)
			}
		}()
	}

	// Send jobs to workers
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

func processJob(job BatchJob, verbose bool, version string) BatchResult {
	result := BatchResult{
		Job:      job,
		Trackers: job.Trackers,
	}

	// Ensure output has .torrent extension
	output := job.Output
	if filepath.Ext(output) != ".torrent" {
		output += ".torrent"
	}

	// Convert job to CreateTorrentOptions
	opts := job.ToCreateOptions(verbose, version)

	// Create the torrent
	mi, err := CreateTorrent(opts)
	if err != nil {
		result.Error = fmt.Errorf("failed to create torrent: %w", err)
		return result
	}

	// Write the torrent file
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

	// Collect torrent info
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

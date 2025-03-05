package display

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/anacrolix/torrent/metainfo"
	"github.com/autobrr/mkbrr/internal/types"
	humanize "github.com/dustin/go-humanize"
	"github.com/fatih/color"
	progressbar "github.com/schollz/progressbar/v3"
)

// BatchJob represents a batch job
type BatchJob struct {
	Path string
}

// BatchResult represents the result of a single job in the batch processing
type BatchResult struct {
	Job      BatchJob
	Success  bool
	Error    error
	Info     *types.TorrentInfo
	Trackers []string
}

type Display struct {
	formatter *Formatter
	bar       *progressbar.ProgressBar
	isBatch   bool
}

// Ensure Display implements all required interfaces
var _ Displayer = (*Display)(nil)
var _ TorrentDisplayer = (*Display)(nil)

func NewDisplay(formatter *Formatter) *Display {
	return &Display{
		formatter: formatter,
	}
}

func (d *Display) ShowProgress(total int) {
	fmt.Println()
	d.bar = progressbar.NewOptions(total,
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionSetDescription("[cyan][bold]Hashing pieces...[reset]"),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
	)
}

func (d *Display) UpdateProgress(completed int, hashrate float64) {
	if d.isBatch {
		return
	}
	if d.bar != nil {
		if err := d.bar.Set(completed); err != nil {
			log.Printf("failed to update progress bar: %v", err)
		}

		if hashrate > 0 {
			description := fmt.Sprintf("[cyan][bold]Hashing pieces...[reset] [%.2f MB/s]", hashrate/1024/1024)
			d.bar.Describe(description)
		}
	}
}

func (d *Display) ShowFiles(files []types.EntryFile) {
	if d.isBatch {
		return
	}

	fmt.Printf("\n%s\n", magenta("Files being hashed:"))
	for i, file := range files {
		prefix := "  ├─"
		if i == len(files)-1 {
			prefix = "  └─"
		}
		fmt.Printf("%s %s (%s)\n",
			prefix,
			success(filepath.Base(file.Path)),
			label(humanize.IBytes(uint64(file.Length))))
	}
	fmt.Println()
}

func (d *Display) FinishProgress() {
	if d.isBatch {
		return
	}
	if d.bar != nil {
		if err := d.bar.Finish(); err != nil {
			log.Printf("failed to finish progress bar: %v", err)
		}
		fmt.Println()
	}
}

func (d *Display) IsBatch() bool {
	return d.isBatch
}

func (d *Display) SetBatch(isBatch bool) {
	d.isBatch = isBatch
}

var (
	magenta    = color.New(color.FgMagenta).SprintFunc()
	green      = color.New(color.FgGreen).SprintFunc()
	yellow     = color.New(color.FgYellow).SprintFunc()
	success    = color.New(color.FgGreen).SprintFunc()
	label      = color.New(color.FgCyan).SprintFunc()
	highlight  = color.New(color.FgHiWhite).SprintFunc()
	errorColor = color.New(color.FgRed).SprintFunc()
	white      = fmt.Sprint
)

func (d *Display) ShowMessage(msg string) {
	fmt.Printf("%s %s\n", success("\nInfo:"), msg)
}

func (d *Display) ShowError(msg string) {
	fmt.Println(errorColor(msg))
}

func (d *Display) ShowTorrentInfo(t *types.Torrent, info interface{}) {
	metaInfo, ok := info.(*metainfo.Info)
	if !ok {
		return
	}
	fmt.Printf("\n%s\n", magenta("Torrent info:"))
	fmt.Printf("  %-13s %s\n", label("Name:"), metaInfo.Name)
	fmt.Printf("  %-13s %s\n", label("Hash:"), t.HashInfoBytes())
	fmt.Printf("  %-13s %s\n", label("Size:"), humanize.IBytes(uint64(metaInfo.TotalLength())))
	fmt.Printf("  %-13s %s\n", label("Piece length:"), humanize.IBytes(uint64(metaInfo.PieceLength)))
	fmt.Printf("  %-13s %d\n", label("Pieces:"), len(metaInfo.Pieces)/20)

	if t.AnnounceList != nil {
		fmt.Printf("  %-13s\n", label("Trackers:"))
		for _, tier := range t.AnnounceList {
			for _, tracker := range tier {
				fmt.Printf("    %s\n", success(tracker))
			}
		}
	} else if t.Announce != "" {
		fmt.Printf("  %-13s %s\n", label("Tracker:"), success(t.Announce))
	}

	if len(t.UrlList) > 0 {
		fmt.Printf("  %-13s\n", label("Web seeds:"))
		for _, seed := range t.UrlList {
			fmt.Printf("    %s\n", highlight(seed))
		}
	}

	if metaInfo.Private != nil && *metaInfo.Private {
		fmt.Printf("  %-13s %s\n", label("Private:"), "yes")
	}

	if metaInfo.Source != "" {
		fmt.Printf("  %-13s %s\n", label("Source:"), metaInfo.Source)
	}

	if t.Comment != "" {
		fmt.Printf("  %-13s %s\n", label("Comment:"), t.Comment)
	}

	if t.CreatedBy != "" {
		fmt.Printf("  %-13s %s\n", label("Created by:"), t.CreatedBy)
	}

	if t.CreationDate != 0 {
		creationTime := time.Unix(t.CreationDate, 0)
		fmt.Printf("  %-13s %s\n", label("Created on:"), creationTime.Format("2006-01-02 15:04:05 MST"))
	}

	if len(metaInfo.Files) > 0 {
		fmt.Printf("  %-13s %d\n", label("Files:"), len(metaInfo.Files))
	}
}

func (d *Display) ShowFileTree(info interface{}) {
	metaInfo, ok := info.(*metainfo.Info)
	if !ok {
		return
	}
	fmt.Printf("\n%s\n", magenta("File tree:"))
	fmt.Printf("%s %s\n", "└─", success(metaInfo.Name))
	for i, file := range metaInfo.Files {
		prefix := "  ├─"
		if i == len(metaInfo.Files)-1 {
			prefix = "  └─"
		}
		fmt.Printf("%s %s (%s)\n",
			prefix,
			success(filepath.Join(file.Path...)),
			label(humanize.IBytes(uint64(file.Length))))
	}
}

func (d *Display) ShowOutputPathWithTime(path string, durationInput interface{}) {
	duration, ok := durationInput.(time.Duration)
	if !ok {
		return
	}

	if duration < time.Second {
		fmt.Printf("\n%s %s (%s)\n",
			success("Wrote"),
			white(path),
			magenta(fmt.Sprintf("elapsed %dms", duration.Milliseconds())))
	} else {
		fmt.Printf("\n%s %s (%s)\n",
			success("Wrote"),
			white(path),
			magenta(fmt.Sprintf("elapsed %.2fs", duration.Seconds())))
	}
}

func (d *Display) ShowBatchResults(resultsInput interface{}, durationInput interface{}) {
	results, ok := resultsInput.([]BatchResult)
	if !ok {
		return
	}

	duration, ok := durationInput.(time.Duration)
	if !ok {
		return
	}

	fmt.Printf("\n%s\n", magenta("Batch processing results:"))

	successful := 0
	failed := 0
	totalSize := int64(0)

	for _, result := range results {
		if result.Success {
			successful++
			if result.Info != nil {
				totalSize += result.Info.Size
			}
		} else {
			failed++
		}
	}

	fmt.Printf("  %-15s %d\n", label("Total jobs:"), len(results))
	fmt.Printf("  %-15s %s\n", label("Successful:"), success(successful))
	fmt.Printf("  %-15s %s\n", label("Failed:"), errorColor(failed))
	fmt.Printf("  %-15s %s\n", label("Total size:"), humanize.IBytes(uint64(totalSize)))
	fmt.Printf("  %-15s %s\n", label("Processing time:"), d.formatter.FormatDuration(duration))

	if d.formatter.verbose {
		fmt.Printf("\n%s\n", magenta("Detailed results:"))
		for i, result := range results {
			fmt.Printf("\n%s %d:\n", label("Job"), i+1)
			if result.Success {
				fmt.Printf("  %-11s %s\n", label("Status:"), success("Success"))
				fmt.Printf("  %-11s %s\n", label("Output:"), result.Info.Path)
				fmt.Printf("  %-11s %s\n", label("Size:"), humanize.IBytes(uint64(result.Info.Size)))
				fmt.Printf("  %-11s %s\n", label("Info hash:"), result.Info.InfoHash)
				fmt.Printf("  %-11s %s\n", label("Trackers:"), strings.Join(result.Trackers, ", "))
				if result.Info.Files > 0 {
					fmt.Printf("  %-11s %d\n", label("Files:"), result.Info.Files)
				}
			} else {
				fmt.Printf("  %-11s %s\n", label("Status:"), errorColor("Failed"))
				fmt.Printf("  %-11s %v\n", label("Error:"), result.Error)
				fmt.Printf("  %-11s %s\n", label("Input:"), result.Job.Path)
			}
		}
	}
}

type Formatter struct {
	verbose bool
}

func NewFormatter(verbose bool) *Formatter {
	return &Formatter{verbose: verbose}
}

func (f *Formatter) FormatBytes(bytes int64) string {
	return humanize.IBytes(uint64(bytes))
}

func (f *Formatter) FormatDuration(dur time.Duration) string {
	if dur < time.Second {
		return fmt.Sprintf("%dms", dur.Milliseconds())
	}
	return humanize.RelTime(time.Now().Add(-dur), time.Now(), "", "")
}

func (d *Display) ShowWarning(msg string) {
	fmt.Printf("%s %s\n", yellow("Warning:"), msg)
}

func (d *Display) ShowSeasonPackWarnings(info *SeasonPackInfo) {
	if !info.IsSeasonPack {
		return
	}

	if info.IsSuspicious || len(info.MissingEpisodes) > 0 {
		fmt.Printf("\n%s %s\n", yellow("Warning:"), "Possible incomplete season pack detected")
		fmt.Printf("  %-13s %d\n", label("Season number:"), info.Season)
		fmt.Printf("  %-13s %d\n", label("Highest episode number found:"), info.MaxEpisode)
		fmt.Printf("  %-13s %d\n", label("Video files:"), info.VideoFileCount)

		fmt.Println(yellow("\nThis may be an incomplete season pack. Check files before uploading."))
	}
}

// NewDisplayer returns a Displayer interface implementation
func NewDisplayer(verbose bool) Displayer {
	return NewDisplay(NewFormatter(verbose))
}

// GetTorrentDisplayer returns a TorrentDisplayer interface implementation
func GetTorrentDisplayer(displayer Displayer) TorrentDisplayer {
	if d, ok := displayer.(*Display); ok {
		return d
	}
	return nil
}

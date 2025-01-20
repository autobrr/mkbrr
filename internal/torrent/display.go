package torrent

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/anacrolix/torrent/metainfo"
	humanize "github.com/dustin/go-humanize"
	"github.com/fatih/color"
	progressbar "github.com/schollz/progressbar/v3"
)

type Display struct {
	formatter *Formatter
}

func NewDisplay(formatter *Formatter) *Display {
	return &Display{formatter: formatter}
}

var (
	bar *progressbar.ProgressBar
)

func (d *Display) ShowProgress(total int) {
	fmt.Println()
	bar = progressbar.NewOptions(total,
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

func (d *Display) UpdateProgress(completed int) {
	if bar != nil {
		bar.Set(completed)
	}
}

func (d *Display) FinishProgress() {
	if bar != nil {
		bar.Finish()
		fmt.Println()
	}
}

var (
	cyan       = color.New(color.FgMagenta, color.Bold).SprintFunc()
	label      = color.New(color.Bold, color.FgHiWhite).SprintFunc()
	success    = color.New(color.FgHiGreen).SprintFunc()
	warning    = color.New(color.FgYellow).SprintFunc()
	errorColor = color.New(color.FgRed).SprintFunc()
	highlight  = color.New(color.FgMagenta).SprintFunc()
	white      = fmt.Sprint
)

func (d *Display) ShowTorrentInfo(t *Torrent, info *metainfo.Info) {
	fmt.Printf("\n%s\n", cyan("Torrent info:"))
	fmt.Printf("  %-13s %s\n", label("Name:"), info.Name)
	fmt.Printf("  %-13s %s\n", label("Hash:"), t.HashInfoBytes())
	fmt.Printf("  %-13s %s\n", label("Length:"), humanize.Bytes(uint64(info.TotalLength())))
	fmt.Printf("  %-13s %s\n", label("Piece length:"), humanize.Bytes(uint64(info.PieceLength)))
	fmt.Printf("  %-13s %d\n", label("Pieces:"), len(info.Pieces)/20)

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

	if info.Private != nil && *info.Private {
		fmt.Printf("  %-13s %s\n", label("Private:"), "yes")
	}

	if info.Source != "" {
		fmt.Printf("  %-13s %s\n", label("Source:"), info.Source)
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

	if len(info.Files) > 0 {
		fmt.Printf("  %-13s %d\n", label("Files:"), len(info.Files))
	}
}

func (d *Display) ShowFileTree(info *metainfo.Info) {
	fmt.Printf("\n%s\n", cyan("File tree:"))
	fmt.Printf("%s %s\n", "└─", success(info.Name))
	for i, file := range info.Files {
		prefix := "  ├─"
		if i == len(info.Files)-1 {
			prefix = "  └─"
		}
		fmt.Printf("%s %s (%s)\n",
			prefix,
			success(filepath.Join(file.Path...)),
			label(humanize.Bytes(uint64(file.Length))))
	}
}

func (d *Display) ShowOutputPathWithTime(path string, duration time.Duration) {
	fmt.Printf("\n%s %s (%s)\n",
		success("Wrote"),
		white(path),
		cyan("took "+d.formatter.FormatDuration(duration)))
}

func (d *Display) ShowBatchResults(results []BatchResult, duration time.Duration) {
	fmt.Printf("\n%s\n", cyan("Batch processing results:"))

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
	fmt.Printf("  %-15s %s\n", label("Total size:"), humanize.Bytes(uint64(totalSize)))
	fmt.Printf("  %-15s %s\n", label("Processing time:"), d.formatter.FormatDuration(duration))

	if d.formatter.verbose {
		fmt.Printf("\n%s\n", cyan("Detailed results:"))
		for i, result := range results {
			fmt.Printf("\n%s %d:\n", label("Job"), i+1)
			if result.Success {
				fmt.Printf("  %-11s %s\n", label("Status:"), success("Success"))
				fmt.Printf("  %-11s %s\n", label("Output:"), result.Info.Path)
				fmt.Printf("  %-11s %s\n", label("Size:"), humanize.Bytes(uint64(result.Info.Size)))
				fmt.Printf("  %-11s %s\n", label("Info hash:"), result.Info.InfoHash)
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
	return humanize.Bytes(uint64(bytes))
}

func (f *Formatter) FormatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return humanize.RelTime(time.Now().Add(-d), time.Now(), "", "")
}

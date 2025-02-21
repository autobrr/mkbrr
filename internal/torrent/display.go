package torrent

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/anacrolix/torrent/metainfo"
	humanize "github.com/dustin/go-humanize"
	"github.com/fatih/color"
	progressbar "github.com/schollz/progressbar/v3"
)

type Display struct {
	formatter *Formatter
	bar       *progressbar.ProgressBar
	isBatch   bool
}

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

func (d *Display) UpdateProgress(completed int) {
	if d.isBatch {
		return
	}
	if d.bar != nil {
		if err := d.bar.Set(completed); err != nil {
			log.Printf("failed to update progress bar: %v", err)
		}
	}
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

func (d *Display) ShowTorrentInfo(t *Torrent, info *metainfo.Info) {
	fmt.Printf("\n%s\n", magenta("Torrent info:"))
	fmt.Printf("  %-13s %s\n", label("Name:"), info.Name)
	fmt.Printf("  %-13s %s\n", label("Hash:"), t.HashInfoBytes())
	fmt.Printf("  %-13s %s\n", label("Size:"), humanize.IBytes(uint64(info.TotalLength())))
	fmt.Printf("  %-13s %s\n", label("Piece length:"), humanize.IBytes(uint64(info.PieceLength)))
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
	fmt.Printf("\n%s\n", magenta("File tree:"))
	fmt.Printf("%s %s\n", "└─", success(info.Name))
	for i, file := range info.Files {
		prefix := "  ├─"
		if i == len(info.Files)-1 {
			prefix = "  └─"
		}
		fmt.Printf("%s %s (%s)\n",
			prefix,
			success(filepath.Join(file.Path...)),
			label(humanize.IBytes(uint64(file.Length))))
	}
}

func (d *Display) ShowOutputPathWithTime(path string, duration time.Duration) {
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

func (d *Display) ShowBatchResults(results []BatchResult, duration time.Duration) {
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

// ShowWarning displays a warning message
func (d *Display) ShowWarning(msg string) {
	fmt.Printf("%s %s\n", yellow("warning:"), msg)
}

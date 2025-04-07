package torrent

import (
	"fmt"
	"io"
	"log"
	"os"
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
	quiet     bool
	output    io.Writer
}

func NewDisplay(formatter *Formatter) *Display {
	return &Display{
		formatter: formatter,
		quiet:     false,
		output:    os.Stdout,
	}
}

// SetQuiet enables/disables quiet mode (output redirected to io.Discard)
func (d *Display) SetQuiet(quiet bool) {
	d.quiet = quiet
	if quiet {
		d.output = io.Discard
	} else {
		d.output = os.Stdout
	}
}

func (d *Display) ShowProgress(total int) {
	// Progress bar needs explicit quiet check because it writes directly to the terminal,
	// bypassing our d.output writer
	if d.quiet {
		return
	}
	fmt.Fprintln(d.output)
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
	// Progress bar needs explicit quiet check because it writes directly to the terminal,
	// bypassing our d.output writer
	if d.isBatch || d.quiet {
		return
	}
	if d.bar != nil {
		if err := d.bar.Set(completed); err != nil {
			log.Printf("failed to update progress bar: %v", err)
		}

		if hashrate > 0 {
			hrStr := d.formatter.FormatBytes(int64(hashrate))
			description := fmt.Sprintf("[cyan][bold]Hashing pieces...[reset] [%s/s]", hrStr)
			d.bar.Describe(description)
		}
	}
}

func (d *Display) ShowFiles(files []fileEntry) {
	fmt.Fprintf(d.output, "\n%s\n", magenta("Files being hashed:"))
	for i, file := range files {
		prefix := "  ├─"
		if i == len(files)-1 {
			prefix = "  └─"
		}
		fmt.Fprintf(d.output, "%s %s (%s)\n",
			prefix,
			success(filepath.Base(file.path)),
			label(d.formatter.FormatBytes(file.length)))
	}
	fmt.Fprintln(d.output)
}

func (d *Display) FinishProgress() {
	// Progress bar needs explicit quiet check because it writes directly to the terminal,
	// bypassing our d.output writer
	if d.quiet {
		return
	}
	if d.bar != nil {
		if err := d.bar.Finish(); err != nil {
			log.Printf("failed to finish progress bar: %v", err)
		}
		fmt.Fprintln(d.output)
	}
}

func (d *Display) IsBatch() bool {
	return d.isBatch
}

func (d *Display) SetBatch(isBatch bool) {
	d.isBatch = isBatch
}

var (
	magenta = color.New(color.FgMagenta).SprintFunc()
	//green      = color.New(color.FgGreen).SprintFunc()
	yellow     = color.New(color.FgYellow).SprintFunc()
	success    = color.New(color.FgGreen).SprintFunc()
	label      = color.New(color.FgCyan).SprintFunc()
	highlight  = color.New(color.FgHiWhite).SprintFunc()
	errorColor = color.New(color.FgRed).SprintFunc()
	white      = fmt.Sprint
)

func (d *Display) ShowMessage(msg string) {
	fmt.Fprintf(d.output, "%s %s\n", success("\nInfo:"), msg)
}

func (d *Display) ShowError(msg string) {
	fmt.Fprintln(d.output, errorColor(msg))
}

func (d *Display) ShowWarning(msg string) {
	fmt.Fprintf(d.output, "%s %s\n", yellow("Warning:"), msg)
}

func (d *Display) ShowTorrentInfo(t *Torrent, info *metainfo.Info) {
	fmt.Fprintf(d.output, "\n%s\n", magenta("Torrent info:"))
	fmt.Fprintf(d.output, "  %-13s %s\n", label("Name:"), info.Name)
	fmt.Fprintf(d.output, "  %-13s %s\n", label("Hash:"), t.HashInfoBytes())
	fmt.Fprintf(d.output, "  %-13s %s\n", label("Size:"), d.formatter.FormatBytes(info.TotalLength()))
	fmt.Fprintf(d.output, "  %-13s %s\n", label("Piece length:"), d.formatter.FormatBytes(info.PieceLength))
	fmt.Fprintf(d.output, "  %-13s %d\n", label("Pieces:"), len(info.Pieces)/20)

	if t.AnnounceList != nil {
		fmt.Fprintf(d.output, "  %-13s\n", label("Trackers:"))
		for _, tier := range t.AnnounceList {
			for _, tracker := range tier {
				fmt.Fprintf(d.output, "    %s\n", success(tracker))
			}
		}
	} else if t.Announce != "" {
		fmt.Fprintf(d.output, "  %-13s %s\n", label("Tracker:"), success(t.Announce))
	}

	if len(t.UrlList) > 0 {
		fmt.Fprintf(d.output, "  %-13s\n", label("Web seeds:"))
		for _, seed := range t.UrlList {
			fmt.Fprintf(d.output, "    %s\n", highlight(seed))
		}
	}

	if info.Private != nil && *info.Private {
		fmt.Fprintf(d.output, "  %-13s %s\n", label("Private:"), "yes")
	}

	if info.Source != "" {
		fmt.Fprintf(d.output, "  %-13s %s\n", label("Source:"), info.Source)
	}

	if t.Comment != "" {
		fmt.Fprintf(d.output, "  %-13s %s\n", label("Comment:"), t.Comment)
	}

	if t.CreatedBy != "" {
		fmt.Fprintf(d.output, "  %-13s %s\n", label("Created by:"), t.CreatedBy)
	}

	if t.CreationDate != 0 {
		creationTime := time.Unix(t.CreationDate, 0)
		fmt.Fprintf(d.output, "  %-13s %s\n", label("Created on:"), creationTime.Format("2006-01-02 15:04:05 MST"))
	}

	if len(info.Files) > 0 {
		fmt.Fprintf(d.output, "  %-13s %d\n", label("Files:"), len(info.Files))
	}
}

func (d *Display) ShowFileTree(info *metainfo.Info) {
	fmt.Fprintf(d.output, "\n%s\n", magenta("File tree:"))
	fmt.Fprintf(d.output, "%s %s\n", "└─", success(info.Name))
	for i, file := range info.Files {
		prefix := "  ├─"
		if i == len(info.Files)-1 {
			prefix = "  └─"
		}
		fmt.Fprintf(d.output, "%s %s (%s)\n",
			prefix,
			success(filepath.Join(file.Path...)),
			label(d.formatter.FormatBytes(file.Length)))
	}
}

func (d *Display) ShowOutputPathWithTime(path string, duration time.Duration) {
	if duration < time.Second {
		fmt.Fprintf(d.output, "\n%s %s (%s)\n",
			success("Wrote"),
			white(path),
			magenta(fmt.Sprintf("elapsed %dms", duration.Milliseconds())))
	} else {
		fmt.Fprintf(d.output, "\n%s %s (%s)\n",
			success("Wrote"),
			white(path),
			magenta(fmt.Sprintf("elapsed %.2fs", duration.Seconds())))
	}
}

func (d *Display) ShowBatchResults(results []BatchResult, duration time.Duration) {
	fmt.Fprintf(d.output, "\n%s\n", magenta("Batch processing results:"))

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

	fmt.Fprintf(d.output, "  %-15s %d\n", label("Total jobs:"), len(results))
	fmt.Fprintf(d.output, "  %-15s %s\n", label("Successful:"), success(successful))
	fmt.Fprintf(d.output, "  %-15s %s\n", label("Failed:"), errorColor(failed))
	fmt.Fprintf(d.output, "  %-15s %s\n", label("Total size:"), d.formatter.FormatBytes(totalSize))
	fmt.Fprintf(d.output, "  %-15s %s\n", label("Processing time:"), d.formatter.FormatDuration(duration))

	if d.formatter.verbose {
		fmt.Fprintf(d.output, "\n%s\n", magenta("Detailed results:"))
		for i, result := range results {
			fmt.Fprintf(d.output, "\n%s %d:\n", label("Job"), i+1)
			if result.Success {
				fmt.Fprintf(d.output, "  %-11s %s\n", label("Status:"), success("Success"))
				fmt.Fprintf(d.output, "  %-11s %s\n", label("Output:"), result.Info.Path)
				fmt.Fprintf(d.output, "  %-11s %s\n", label("Size:"), d.formatter.FormatBytes(result.Info.Size))
				fmt.Fprintf(d.output, "  %-11s %s\n", label("Info hash:"), result.Info.InfoHash)
				fmt.Fprintf(d.output, "  %-11s %s\n", label("Trackers:"), strings.Join(result.Trackers, ", "))
				if result.Info.Files > 0 {
					fmt.Fprintf(d.output, "  %-11s %d\n", label("Files:"), result.Info.Files)
				}
			} else {
				fmt.Fprintf(d.output, "  %-11s %s\n", label("Status:"), errorColor("Failed"))
				fmt.Fprintf(d.output, "  %-11s %v\n", label("Error:"), result.Error)
				fmt.Fprintf(d.output, "  %-11s %s\n", label("Input:"), result.Job.Path)
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

func (d *Display) ShowSeasonPackWarnings(info *SeasonPackInfo) {
	if !info.IsSeasonPack {
		return
	}

	if len(info.MissingEpisodes) > 0 {
		fmt.Fprintf(d.output, "\n%s %s\n", yellow("Warning:"), "Possible incomplete season pack detected")
		fmt.Fprintf(d.output, "  %-13s %d\n", label("Season number:"), info.Season)
		fmt.Fprintf(d.output, "  %-13s %d\n", label("Highest episode number found:"), info.MaxEpisode)
		fmt.Fprintf(d.output, "  %-13s %d\n", label("Episodes found:"), len(info.Episodes))

		missingStrs := make([]string, len(info.MissingEpisodes))
		for i, ep := range info.MissingEpisodes {
			missingStrs[i] = fmt.Sprintf("episode %d", ep)
		}
		fmt.Fprintf(d.output, "  %-13s %s\n", label("Missing:"), strings.Join(missingStrs, ", "))

		fmt.Fprintln(d.output, yellow("\nThis may be an incomplete season pack. Check files before uploading."))
	}
}

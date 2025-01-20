package torrent

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/anacrolix/torrent/metainfo"
	"github.com/dustin/go-humanize"
	"github.com/fatih/color"
	"github.com/schollz/progressbar/v3"
)

type Displayer interface {
	ShowProgress(total int) *progressbar.ProgressBar
	UpdateProgress(count int)
	FinishProgress()
}

type Formatter struct {
	verbose bool
	colored bool
}

// NewFormatter creates a new Formatter with the given verbosity.
func NewFormatter(verbose bool) *Formatter {
	return &Formatter{
		verbose: verbose,
		colored: true, // enabled by default, could be made configurable
	}
}

// FormatTorrentInfo returns a formatted string of torrent information.
func (f *Formatter) FormatTorrentInfo(t interface{}, info *metainfo.Info) (string, error) {
	var mi *metainfo.MetaInfo
	switch v := t.(type) {
	case *metainfo.MetaInfo:
		mi = v
	case *Torrent:
		mi = v.MetaInfo
	default:
		return "", fmt.Errorf("unsupported type: %T", t)
	}

	var buffer bytes.Buffer

	labelColor := color.New(color.FgCyan).SprintFunc()
	valueColor := color.New(color.FgWhite).SprintFunc()

	buffer.WriteString(fmt.Sprintf("\n\n%s %s\n", labelColor("Name:"), valueColor(info.Name)))
	buffer.WriteString(fmt.Sprintf("%s %s\n", labelColor("Size:"), valueColor(humanize.Bytes(uint64(info.TotalLength())))))
	buffer.WriteString(fmt.Sprintf("%s %s\n", labelColor("Hash:"), valueColor(mi.HashInfoBytes().String())))

	if f.verbose {
		buffer.WriteString(fmt.Sprintf("%s %s\n", labelColor("Pieces:"), valueColor(fmt.Sprintf("%d", len(info.Pieces)/20))))
		buffer.WriteString(fmt.Sprintf("%s %s\n", labelColor("Piece Length:"), valueColor(humanize.Bytes(uint64(info.PieceLength)))))

		private := false
		if info.Private != nil {
			private = *info.Private
		}
		buffer.WriteString(fmt.Sprintf("%s %v\n", labelColor("Private:"), valueColor(fmt.Sprintf("%v", private))))

		if mi.Comment != "" {
			buffer.WriteString(fmt.Sprintf("%s %s\n", labelColor("Comment:"), valueColor(mi.Comment)))
		}
		if mi.Announce != "" {
			buffer.WriteString(fmt.Sprintf("%s %s\n", labelColor("Tracker:"), valueColor(mi.Announce)))
		}

		if mi.CreatedBy != "" {
			buffer.WriteString(fmt.Sprintf("%s %s\n", labelColor("Created by:"), valueColor(mi.CreatedBy)))
		}
		if mi.CreationDate != 0 {
			creationTime := time.Unix(mi.CreationDate, 0)
			buffer.WriteString(fmt.Sprintf("%s %s\n", labelColor("Created:"), valueColor(creationTime.Format(time.RFC1123))))
		}

		magnet, err := mi.MagnetV2()
		if err == nil {
			buffer.WriteString(fmt.Sprintf("%s %s\n", labelColor("Magnet Link:"), valueColor(magnet)))
		}
	}

	return buffer.String(), nil
}

// FormatFileTree returns a formatted string of the torrent's file tree.
func (f *Formatter) FormatFileTree(info *metainfo.Info) (string, error) {
	if len(info.Files) == 0 {
		return "", nil
	}

	var buffer bytes.Buffer
	dirColor := color.New(color.FgYellow).SprintFunc()
	fileColor := color.New(color.FgWhite).SprintFunc()
	sizeColor := color.New(color.FgCyan).SprintFunc()

	buffer.WriteString(fmt.Sprintf("\n%s  %s\n", dirColor("Files"), info.Name))

	filesByPath := make(map[string][]FileEntry)
	for _, fEntry := range info.Files {
		path := strings.Join(fEntry.Path, string(filepath.Separator))
		dir := filepath.Dir(path)
		if dir == "." {
			dir = ""
		}
		filesByPath[dir] = append(filesByPath[dir], FileEntry{
			Name: filepath.Base(path),
			Size: fEntry.Length,
			Path: path,
		})
	}

	prefix := "       " // 7 spaces to align with "Files  "
	for dir, files := range filesByPath {
		if dir != "" {
			buffer.WriteString(fmt.Sprintf("%s├─%s\n", prefix, dirColor(dir)))
			for i, file := range files {
				var connector string
				if i == len(files)-1 {
					connector = "└─"
				} else {
					connector = "├─"
				}
				buffer.WriteString(fmt.Sprintf("%s│  %s%s [%s]\n",
					prefix,
					connector,
					fileColor(file.Name),
					sizeColor(humanize.Bytes(uint64(file.Size)))))
			}
		} else {
			for i, file := range files {
				var connector string
				if i == len(files)-1 {
					connector = "└─"
				} else {
					connector = "├─"
				}
				buffer.WriteString(fmt.Sprintf("%s%s%s [%s]\n",
					prefix,
					connector,
					fileColor(file.Name),
					sizeColor(humanize.Bytes(uint64(file.Size)))))
			}
		}
	}

	return buffer.String(), nil
}

// Display handles outputting formatted torrent information to the console.
type Display struct {
	formatter *Formatter
	progress  *progressbar.ProgressBar
}

// NewDisplay creates a new Display instance.
func NewDisplay(formatter *Formatter) *Display {
	return &Display{formatter: formatter}
}

func (d *Display) ShowTorrentInfo(t interface{}, info *metainfo.Info) {
	formatted, err := d.formatter.FormatTorrentInfo(t, info)
	if err != nil {
		fmt.Printf("error formatting torrent info: %v\n", err)
		return
	}
	fmt.Print(formatted)
}

func (d *Display) ShowFileTree(info *metainfo.Info) {
	formatted, err := d.formatter.FormatFileTree(info)
	if err != nil {
		fmt.Printf("error formatting file tree: %v\n", err)
		return
	}
	fmt.Print(formatted)
}

func (d *Display) ShowOutputPath(path string) {
	successColor := color.New(color.FgGreen).SprintFunc()
	valueColor := color.New(color.FgWhite).SprintFunc()
	fmt.Printf("\n%s %s\n", successColor("Output:"), valueColor(path))
}

func (d *Display) ShowProgress(total int) *progressbar.ProgressBar {
	fmt.Print("\n")
	d.progress = progressbar.NewOptions(total,
		progressbar.OptionSetDescription("Hashing pieces"),
		progressbar.OptionSetItsString("piece"),
		progressbar.OptionSetWidth(15),
		progressbar.OptionThrottle(65*time.Millisecond),
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[cyan]=[reset]",
			SaucerHead:    "[cyan]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
		progressbar.OptionSetDescription("[cyan]Hashing pieces[reset]"),
	)
	return d.progress
}

func (d *Display) UpdateProgress(count int) {
	if d.progress != nil {
		d.progress.Set(count)
	}
}

func (d *Display) FinishProgress() {
	if d.progress != nil {
		d.progress.Finish()
	}
}

func (d *Display) ShowOutputPathWithTime(path string, duration time.Duration) {
	successColor := color.New(color.FgGreen).SprintFunc()
	valueColor := color.New(color.FgWhite).SprintFunc()
	fmt.Printf("\n%s %s [%s]\n", successColor("Output:"), valueColor(path), successColor(duration.Round(time.Millisecond)))
}

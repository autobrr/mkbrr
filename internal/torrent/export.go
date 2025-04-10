package torrent

import (
	"encoding/json"
	"path/filepath"

	"github.com/anacrolix/torrent/bencode"
	"github.com/anacrolix/torrent/metainfo"
)

// FileDetail holds structured information about a single file within a torrent.
type FileDetail struct {
	Path            string `json:"path"`
	Length          int64  `json:"length"`
	LengthFormatted string `json:"lengthFormatted"`
}

// TorrentInspectJSON holds all the structured information for JSON output.
type TorrentInspectJSON struct {
	Name                 string                 `json:"name"`
	InfoHash             string                 `json:"infoHash"`
	Size                 int64                  `json:"size"`
	SizeFormatted        string                 `json:"sizeFormatted"`
	PieceLength          int64                  `json:"pieceLength"`
	PieceLengthFormatted string                 `json:"pieceLengthFormatted"`
	NumPieces            int                    `json:"numPieces"`
	IsPrivate            *bool                  `json:"isPrivate,omitempty"`
	Source               string                 `json:"source,omitempty"`
	Comment              string                 `json:"comment,omitempty"`
	CreatedBy            string                 `json:"createdBy,omitempty"`
	CreationDate         *int64                 `json:"creationDate,omitempty"`
	Trackers             [][]string             `json:"trackers,omitempty"`
	WebSeeds             []string               `json:"webSeeds,omitempty"`
	Files                []FileDetail           `json:"files,omitempty"`
	AdditionalRootMeta   map[string]interface{} `json:"additionalRootMeta,omitempty"`
	AdditionalInfoMeta   map[string]interface{} `json:"additionalInfoMeta,omitempty"`
	ValidationResults    []ValidationResult     `json:"validationResults,omitempty"`
}

// GenerateInspectJSON gathers torrent information and populates the TorrentInspectJSON struct.
func GenerateInspectJSON(mi *metainfo.MetaInfo, info *metainfo.Info, rawTorrentBytes []byte, verbose bool, validationResults []ValidationResult) (*TorrentInspectJSON, error) {
	formatter := NewFormatter(verbose)

	output := &TorrentInspectJSON{
		Name:                 info.Name,
		InfoHash:             mi.HashInfoBytes().String(),
		Size:                 info.TotalLength(),
		SizeFormatted:        formatter.FormatBytes(info.TotalLength()),
		PieceLength:          info.PieceLength,
		PieceLengthFormatted: formatter.FormatBytes(info.PieceLength),
		NumPieces:            len(info.Pieces) / 20,
		IsPrivate:            info.Private,
		Source:               info.Source,
		Comment:              mi.Comment,
		CreatedBy:            mi.CreatedBy,
		Trackers:             mi.AnnounceList,
		WebSeeds:             mi.UrlList,
		ValidationResults:    validationResults, // Assign validation results
	}

	if mi.CreationDate != 0 {
		ts := mi.CreationDate
		output.CreationDate = &ts
	}

	if len(info.Files) > 0 {
		output.Files = make([]FileDetail, len(info.Files))
		for i, f := range info.Files {
			output.Files[i] = FileDetail{
				Path:            filepath.Join(f.Path...),
				Length:          f.Length,
				LengthFormatted: formatter.FormatBytes(f.Length),
			}
		}
	} else if info.Length > 0 {
		output.Files = []FileDetail{
			{
				Path:            info.Name,
				Length:          info.Length,
				LengthFormatted: formatter.FormatBytes(info.Length),
			},
		}
	}

	if verbose {
		rootMap := make(map[string]interface{})
		if err := bencode.Unmarshal(rawTorrentBytes, &rootMap); err == nil {
			standardRoot := map[string]bool{
				"announce": true, "announce-list": true, "comment": true,
				"created by": true, "creation date": true, "info": true,
				"url-list": true, "nodes": true,
			}
			output.AdditionalRootMeta = make(map[string]interface{})
			for k, v := range rootMap {
				if !standardRoot[k] {
					output.AdditionalRootMeta[k] = v
				}
			}
			if len(output.AdditionalRootMeta) == 0 {
				output.AdditionalRootMeta = nil // Don't include empty map
			}
		}

		infoMap := make(map[string]interface{})
		if err := bencode.Unmarshal(mi.InfoBytes, &infoMap); err == nil {
			standardInfo := map[string]bool{
				"name": true, "piece length": true, "pieces": true,
				"files": true, "length": true, "private": true,
				"source": true, "path": true,
			}
			output.AdditionalInfoMeta = make(map[string]interface{})
			for k, v := range infoMap {
				if !standardInfo[k] {
					output.AdditionalInfoMeta[k] = v
				}
			}
			if len(output.AdditionalInfoMeta) == 0 {
				output.AdditionalInfoMeta = nil // Don't include empty map
			}
		}
	}

	return output, nil
}

// ToJSON marshals the TorrentInspectJSON struct into a JSON string.
func (t *TorrentInspectJSON) ToJSON() (string, error) {
	bytes, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

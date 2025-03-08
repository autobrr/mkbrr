package torrentutils

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/anacrolix/torrent/bencode"
	"github.com/anacrolix/torrent/metainfo"
)

// LoadFromFile loads a torrent file and returns a MetaInfo
func LoadFromFile(path string) (*metainfo.MetaInfo, error) {
	mi, err := metainfo.LoadFromFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not load torrent: %w", err)
	}
	return mi, nil
}

// SaveToFile saves a MetaInfo to a file at the specified path
func SaveToFile(mi *metainfo.MetaInfo, path string) error {
	// ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("could not create directory: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("could not create output file: %w", err)
	}
	defer f.Close()

	if err := mi.Write(f); err != nil {
		return fmt.Errorf("could not write output file: %w", err)
	}

	return nil
}

// UpdateTrackers sets the announce URL and announce list for a MetaInfo
func UpdateTrackers(mi *metainfo.MetaInfo, trackerURL string) {
	mi.Announce = trackerURL
	mi.AnnounceList = [][]string{{trackerURL}}
}

// UpdateWebSeeds sets the URL list for a MetaInfo
func UpdateWebSeeds(mi *metainfo.MetaInfo, webSeeds []string) {
	mi.UrlList = webSeeds
}

// UpdateComment sets the comment for a MetaInfo
func UpdateComment(mi *metainfo.MetaInfo, comment string) {
	mi.Comment = comment
}

// UpdateCreatorAndDate sets the creator and creation date
func UpdateCreatorAndDate(mi *metainfo.MetaInfo, creator string, noCreator bool, noDate bool, currentTime int64) {
	if !noCreator {
		mi.CreatedBy = creator
	} else {
		mi.CreatedBy = ""
	}

	if !noDate {
		mi.CreationDate = currentTime
	} else {
		mi.CreationDate = 0
	}
}

// UpdatePrivateFlag sets the private flag in a MetaInfo's info dictionary
func UpdatePrivateFlag(mi *metainfo.MetaInfo, isPrivate *bool) (bool, error) {
	if isPrivate == nil {
		return false, nil
	}

	info, err := mi.UnmarshalInfo()
	if err != nil {
		return false, fmt.Errorf("could not unmarshal info: %w", err)
	}

	// Update only if different
	if info.Private == nil || *info.Private != *isPrivate {
		info.Private = isPrivate
		infoBytes, err := bencode.Marshal(info)
		if err != nil {
			return false, fmt.Errorf("could not marshal info: %w", err)
		}
		mi.InfoBytes = infoBytes
		return true, nil
	}

	return false, nil
}

// UpdateSource sets the source field in a MetaInfo's info dictionary
func UpdateSource(mi *metainfo.MetaInfo, source string) (bool, error) {
	if source == "" {
		return false, nil
	}

	info, err := mi.UnmarshalInfo()
	if err != nil {
		return false, fmt.Errorf("could not unmarshal info: %w", err)
	}

	if info.Source != source {
		info.Source = source
		infoBytes, err := bencode.Marshal(info)
		if err != nil {
			return false, fmt.Errorf("could not marshal info: %w", err)
		}
		mi.InfoBytes = infoBytes
		return true, nil
	}

	return false, nil
}

// GetInfoName extracts the name from MetaInfo
func GetInfoName(mi *metainfo.MetaInfo) (string, error) {
	info, err := mi.UnmarshalInfo()
	if err != nil {
		return "", fmt.Errorf("could not unmarshal info: %w", err)
	}
	return info.Name, nil
}

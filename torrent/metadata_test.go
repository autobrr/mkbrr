// Copyright (c) 2025-2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package torrent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/autobrr/go-torrent/bencode"
	"github.com/autobrr/go-torrent/metainfo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testMetaInfo(t *testing.T, info map[string]any) *metainfo.MetaInfo {
	t.Helper()
	infoBytes, err := bencode.Marshal(info)
	require.NoError(t, err)
	return &metainfo.MetaInfo{InfoBytes: infoBytes}
}

func infoMap(t *testing.T, mi *metainfo.MetaInfo) map[string]any {
	t.Helper()
	m := make(map[string]any)
	require.NoError(t, bencode.Unmarshal(mi.InfoBytes, &m))
	return m
}

func TestApplyMetadata(t *testing.T) {
	type state struct {
		announce     string
		announceList metainfo.AnnounceList
		comment      string
		createdBy    string
		creationDate int64
		urlList      metainfo.UrlList
		info         map[string]any
	}
	full := state{
		announce:     "https://a.test/announce",
		announceList: metainfo.AnnounceList{{"https://a.test/announce"}, {"https://b.test/announce"}},
		comment:      "c",
		createdBy:    "other",
		creationDate: 42,
		info:         map[string]any{"name": "test", "private": int64(1), "source": "SRC", "entropy": "e", "custom": "x"},
	}

	tests := []struct {
		name        string
		before      state
		spec        metadataSpec
		wantChanged bool
		want        state
	}{
		{
			name:        "empty spec keeps every field",
			before:      full,
			spec:        metadataSpec{},
			wantChanged: false,
			want:        full,
		},
		{
			name:   "one tracker removes the announce-list",
			before: full,
			spec:   metadataSpec{Trackers: setTo([]string{"https://new.test/announce"})},
			want: func() state {
				s := full
				s.announce, s.announceList = "https://new.test/announce", nil
				return s
			}(),
			wantChanged: true,
		},
		{
			name:   "several trackers write an announce-list",
			before: state{info: map[string]any{"name": "test"}},
			spec:   metadataSpec{Trackers: setTo([]string{"https://a.test/announce", "https://b.test/announce"})},
			want: state{
				announce:     "https://a.test/announce",
				announceList: metainfo.AnnounceList{{"https://a.test/announce"}, {"https://b.test/announce"}},
				info:         map[string]any{"name": "test"},
			},
			wantChanged: true,
		},
		{
			name:   "same values are no change",
			before: full,
			spec: metadataSpec{
				Trackers:  setTo([]string{"https://a.test/announce", "https://b.test/announce"}),
				Comment:   setTo("c"),
				CreatedBy: setTo("other"),
				Name:      setTo("test"),
				Private:   setTo(true),
				Source:    setTo("SRC"),
			},
			wantChanged: false,
			want:        full,
		},
		{
			name:   "clear removes fields and keeps unknown info keys",
			before: full,
			spec: metadataSpec{
				Trackers:     cleared[[]string](),
				Comment:      cleared[string](),
				CreatedBy:    cleared[string](),
				CreationDate: cleared[int64](),
				Private:      cleared[bool](),
				Source:       cleared[string](),
				Entropy:      clearField,
			},
			wantChanged: true,
			want:        state{info: map[string]any{"name": "test", "custom": "x"}},
		},
		{
			name:        "clearing absent fields is no change",
			before:      state{info: map[string]any{"name": "test"}},
			spec:        metadataSpec{Comment: cleared[string](), Private: cleared[bool](), Source: cleared[string]()},
			wantChanged: false,
			want:        state{info: map[string]any{"name": "test"}},
		},
		{
			name:        "clearing an empty source key is a change",
			before:      state{info: map[string]any{"name": "test", "source": ""}},
			spec:        metadataSpec{Source: cleared[string]()},
			wantChanged: true,
			want:        state{info: map[string]any{"name": "test"}},
		},
		{
			name:        "set info fields",
			before:      state{info: map[string]any{"name": "old", "private": int64(1)}},
			spec:        metadataSpec{Name: setTo("new"), Private: setTo(false), Source: setTo("SRC")},
			wantChanged: true,
			want:        state{info: map[string]any{"name": "new", "private": int64(0), "source": "SRC"}},
		},
		{
			name:        "set web seeds",
			before:      state{info: map[string]any{"name": "test"}},
			spec:        metadataSpec{WebSeeds: []string{"https://seed.test/"}},
			wantChanged: true,
			want:        state{urlList: metainfo.UrlList{"https://seed.test/"}, info: map[string]any{"name": "test"}},
		},
		{
			name:        "same web seeds are no change",
			before:      state{urlList: metainfo.UrlList{"https://seed.test/"}, info: map[string]any{"name": "test"}},
			spec:        metadataSpec{WebSeeds: []string{"https://seed.test/"}},
			wantChanged: false,
			want:        state{urlList: metainfo.UrlList{"https://seed.test/"}, info: map[string]any{"name": "test"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mi := testMetaInfo(t, tt.before.info)
			mi.Announce = tt.before.announce
			mi.AnnounceList = tt.before.announceList
			mi.Comment = tt.before.comment
			mi.CreatedBy = tt.before.createdBy
			mi.CreationDate = tt.before.creationDate
			mi.UrlList = tt.before.urlList
			infoBefore := string(mi.InfoBytes)

			changed, err := applyMetadata(mi, tt.spec)
			require.NoError(t, err)
			assert.Equal(t, tt.wantChanged, changed)

			got := state{mi.Announce, mi.AnnounceList, mi.Comment, mi.CreatedBy, mi.CreationDate, mi.UrlList, infoMap(t, mi)}
			assert.Equal(t, tt.want, got)
			if !changed {
				assert.Equal(t, infoBefore, string(mi.InfoBytes), "info dictionary was rewritten")
			}
		})
	}
}

func TestApplyMetadata_EntropyReplacesExisting(t *testing.T) {
	mi := testMetaInfo(t, map[string]any{"name": "test", "entropy": "old"})

	changed, err := applyMetadata(mi, metadataSpec{Entropy: setField})
	require.NoError(t, err)
	assert.True(t, changed)
	entropy, _ := infoMap(t, mi)["entropy"].(string)
	assert.NotEmpty(t, entropy)
	assert.NotEqual(t, "old", entropy)
}

func TestApplyMetadata_InvalidInfoReturnsError(t *testing.T) {
	mi := &metainfo.MetaInfo{InfoBytes: []byte("not bencode")}
	_, err := applyMetadata(mi, metadataSpec{Source: setTo("SRC")})
	assert.Error(t, err)
}

func TestCreatedBy(t *testing.T) {
	assert.Equal(t, "mkbrr/1.2.3 (https://github.com/autobrr/mkbrr)", createdBy("1.2.3"))
	assert.Equal(t, "mkbrr/dev (https://github.com/autobrr/mkbrr)", createdBy(""))
}

// The writer edits the info dictionary as a map. The bytes must stay the same
// as the typed encoding, or create would give a different info hash than before.
func TestCreateTorrent_InfoBytesMatchTypedEncoding(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.txt")
	require.NoError(t, os.WriteFile(path, []byte("info hash stability"), 0o644))

	for _, private := range []bool{true, false} {
		tor, err := CreateTorrent(CreateOptions{Path: path, IsPrivate: private, Source: "SRC", Quiet: true})
		require.NoError(t, err)
		info, err := tor.UnmarshalInfo()
		require.NoError(t, err)
		typed, err := bencode.Marshal(info)
		require.NoError(t, err)
		assert.Equal(t, string(typed), string(tor.InfoBytes), "private = %v", private)
	}
}

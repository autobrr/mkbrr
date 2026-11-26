// Copyright (c) 2025-2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package torrent

import (
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func failTorrentWrites(t *testing.T) {
	t.Helper()
	prev := openTempTorrentFile
	t.Cleanup(func() { openTempTorrentFile = prev })
	openTempTorrentFile = func(name string, flag int, perm os.FileMode) (*os.File, error) {
		f, err := prev(name, flag, perm)
		if err != nil {
			return nil, err
		}
		if err := f.Close(); err != nil {
			return nil, err
		}
		return os.Open(name)
	}
}

func assertFolderHolds(t *testing.T, dir string, want ...string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	got := make([]string, 0, len(entries))
	for _, entry := range entries {
		got = append(got, entry.Name())
	}
	assert.ElementsMatch(t, want, got)
}

func TestModifyKeepsOriginalWhenWriteFails(t *testing.T) {
	contentDir := t.TempDir()
	content := filepath.Join(contentDir, "file.txt")
	require.NoError(t, os.WriteFile(content, []byte("modify write failure"), 0o644))

	tests := []struct {
		name string
		opts func(dir string) ModifyOptions
		out  string
	}{
		{
			name: "output is the input file",
			opts: func(dir string) ModifyOptions {
				return ModifyOptions{ModifySettings: ModifySettings{Source: new("NEW"), OutputDir: dir, SkipPrefix: true}}
			},
			out: "file.txt.torrent",
		},
		{
			name: "output replaces another torrent",
			opts: func(dir string) ModifyOptions {
				return ModifyOptions{ModifySettings: ModifySettings{Source: new("NEW"), OutputDir: dir, OutputPattern: "other"}}
			},
			out: "other.torrent",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			input := filepath.Join(dir, "file.txt.torrent")
			_, err := Create(CreateOptions{Path: content, OutputPath: input, NoDate: true, Quiet: true})
			require.NoError(t, err)
			output := filepath.Join(dir, tt.out)
			if output != input {
				require.NoError(t, os.WriteFile(output, oldTorrentBytes, 0o644))
			}
			want, err := os.ReadFile(output)
			require.NoError(t, err)
			failTorrentWrites(t)

			result, err := ModifyTorrent(input, tt.opts(dir))

			require.Error(t, err)
			assert.Equal(t, output, result.OutputPath)
			got, readErr := os.ReadFile(output)
			require.NoError(t, readErr)
			assert.Equal(t, want, got)
			assertFolderHolds(t, dir, slices.Compact([]string{"file.txt.torrent", tt.out})...)
		})
	}
}

func TestUpdateKeepsOriginalWhenWriteFails(t *testing.T) {
	content := writeCancelContent(t, 64<<10)
	seed := filepath.Join(t.TempDir(), "seed.torrent")
	_, err := Create(CreateOptions{Path: content, OutputPath: seed, PieceLengthExp: new(uint(15)), NoDate: true, Quiet: true})
	require.NoError(t, err)
	seedBytes, err := os.ReadFile(seed)
	require.NoError(t, err)

	tests := []struct {
		name    string
		inPlace bool
		out     string
	}{
		{name: "in place", inPlace: true, out: "release.torrent"},
		{name: "output replaces another torrent", out: "other.torrent"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			input := filepath.Join(dir, "release.torrent")
			require.NoError(t, os.WriteFile(input, seedBytes, 0o644))
			output := filepath.Join(dir, tt.out)
			if output != input {
				require.NoError(t, os.WriteFile(output, oldTorrentBytes, 0o644))
			}
			want, err := os.ReadFile(output)
			require.NoError(t, err)
			opts := UpdateOptions{TorrentPath: input, ContentPath: content, InPlace: tt.inPlace, Quiet: true}
			if !tt.inPlace {
				opts.OutputPath = output
			}
			failTorrentWrites(t)

			_, err = UpdateTorrent(opts)

			require.Error(t, err)
			got, readErr := os.ReadFile(output)
			require.NoError(t, readErr)
			assert.Equal(t, want, got)
			assertFolderHolds(t, dir, slices.Compact([]string{"release.torrent", tt.out})...)
		})
	}
}

func TestPrepareCreateContextIgnoresUpdateTempFile(t *testing.T) {
	rootDir := t.TempDir()
	outSubDir := filepath.Join(rootDir, "out")
	require.NoError(t, os.MkdirAll(outSubDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(rootDir, "movie.bin"), []byte("payload"), 0o644))

	content := writeCancelContent(t, 32<<10)
	input := filepath.Join(outSubDir, "release.torrent")
	_, err := Create(CreateOptions{Path: content, OutputPath: input, PieceLengthExp: new(uint(15)), NoDate: true, Quiet: true})
	require.NoError(t, err)

	ctx := t.Context()
	var wg sync.WaitGroup
	t.Cleanup(wg.Wait)
	updateErr := make(chan error, 1)
	wg.Go(func() {
		for ctx.Err() == nil {
			if _, err := UpdateTorrent(UpdateOptions{TorrentPath: input, ContentPath: content, InPlace: true, Quiet: true}); err != nil {
				updateErr <- err
				return
			}
		}
	})

	for i := range 400 {
		select {
		case err := <-updateErr:
			t.Fatalf("update failed: %v", err)
		default:
		}
		prepared, err := prepareCreateContext(ctx, CreateOptions{
			Path:      rootDir,
			NoCreator: true,
			NoDate:    true,
		}, createTorrentOptions{})
		require.NoErrorf(t, err, "iteration %d failed while walking an update temp file", i)
		require.Len(t, prepared.files, 1)
	}
}

func TestUpdateInPlaceKeepsReadOnlyTorrent(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write a read-only file")
	}
	content := writeCancelContent(t, 64<<10)
	dir := t.TempDir()
	input := filepath.Join(dir, "release.torrent")
	_, err := Create(CreateOptions{Path: content, OutputPath: input, PieceLengthExp: new(uint(15)), NoDate: true, Quiet: true})
	require.NoError(t, err)
	require.NoError(t, os.Chmod(input, 0o444))
	want, err := os.ReadFile(input)
	require.NoError(t, err)

	_, err = UpdateTorrent(UpdateOptions{TorrentPath: input, ContentPath: content, InPlace: true, Quiet: true})

	require.ErrorIs(t, err, os.ErrPermission)
	got, readErr := os.ReadFile(input)
	require.NoError(t, readErr)
	assert.Equal(t, want, got)
	assertFolderHolds(t, dir, "release.torrent")
}

func TestModifyReplacesOutputSymlinkWithoutWritingTarget(t *testing.T) {
	dir := t.TempDir()
	content := filepath.Join(dir, "file.txt")
	require.NoError(t, os.WriteFile(content, []byte("modify symlink"), 0o644))
	input := filepath.Join(dir, "in.torrent")
	_, err := Create(CreateOptions{Path: content, OutputPath: input, NoDate: true, Quiet: true})
	require.NoError(t, err)
	target := filepath.Join(t.TempDir(), "target.torrent")
	require.NoError(t, os.WriteFile(target, oldTorrentBytes, 0o644))
	output := filepath.Join(dir, "linked.torrent")
	if err := os.Symlink(target, output); err != nil {
		t.Skipf("symbolic links unavailable: %v", err)
	}

	_, err = ModifyTorrent(input, ModifyOptions{ModifySettings: ModifySettings{Source: new("NEW"), OutputDir: dir, OutputPattern: "linked"}})
	require.NoError(t, err)

	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, oldTorrentBytes, got)
	outputInfo, err := os.Lstat(output)
	require.NoError(t, err)
	assert.True(t, outputInfo.Mode().IsRegular())
}

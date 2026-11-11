// Copyright (c) 2025-2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package torrent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/autobrr/go-torrent/metainfo"
)

type progressCall struct{ completed, total int }

// progressRecorder records progress callbacks and flags any callback after the
// call under test returned.
type progressRecorder struct {
	mu       sync.Mutex
	calls    []progressCall
	returned atomic.Bool
	late     atomic.Int32
	onCall   func(completed, total int)
}

func (r *progressRecorder) callback(completed, total int, _ float64) {
	if r.returned.Load() {
		r.late.Add(1)
	}
	r.mu.Lock()
	r.calls = append(r.calls, progressCall{completed, total})
	r.mu.Unlock()
	if r.onCall != nil {
		r.onCall(completed, total)
	}
}

func (r *progressRecorder) finish(t *testing.T) {
	t.Helper()
	r.returned.Store(true)
	time.Sleep(4 * progressInterval)
	if n := r.late.Load(); n > 0 {
		t.Errorf("got %d progress callbacks after the call returned", n)
	}
}

func (r *progressRecorder) snapshot() []progressCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.calls)
}

// writeCancelContent writes a file with many 32 KiB pieces, so that hashing
// takes several reads per worker.
func writeCancelContent(t *testing.T, size int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "content.bin")
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i * 31)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func cancelCreateOptions(t *testing.T, content string) CreateOptions {
	t.Helper()
	return CreateOptions{
		Path:           content,
		OutputPath:     filepath.Join(t.TempDir(), "out.torrent"),
		PieceLengthExp: new(uint(15)),
		NoDate:         true,
		NoCreator:      true,
		Quiet:          true,
		Workers:        1,
	}
}

func cancelVerifyOptions(t *testing.T, content string) VerifyOptions {
	t.Helper()
	opts := cancelCreateOptions(t, content)
	if _, err := Create(opts); err != nil {
		t.Fatal(err)
	}
	return VerifyOptions{TorrentPath: opts.OutputPath, ContentPath: content, Quiet: true, Workers: 1}
}

func assertNoFile(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("output file %s: want not exist, got stat error %v", path, err)
	}
}

func TestContextFunctionsReturnCancelErrorWhenAlreadyDone(t *testing.T) {
	content := writeCancelContent(t, 64<<10)
	verifyOpts := cancelVerifyOptions(t, content)

	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	expired, cancelExpired := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
	defer cancelExpired()

	for _, tc := range []struct {
		name string
		ctx  context.Context
		want error
	}{
		{"canceled", canceled, context.Canceled},
		{"deadline", expired, context.DeadlineExceeded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var rec progressRecorder
			createOpts := cancelCreateOptions(t, content)
			createOpts.ProgressCallback = rec.callback

			if _, err := CreateContext(tc.ctx, createOpts); !errors.Is(err, tc.want) {
				t.Errorf("CreateContext error = %v, want %v", err, tc.want)
			}
			assertNoFile(t, createOpts.OutputPath)

			if tr, err := CreateTorrentContext(tc.ctx, createOpts); tr != nil || !errors.Is(err, tc.want) {
				t.Errorf("CreateTorrentContext = %v, %v, want nil, %v", tr, err, tc.want)
			}

			verifyOpts.ProgressCallback = rec.callback
			if res, err := VerifyDataContext(tc.ctx, verifyOpts); res != nil || !errors.Is(err, tc.want) {
				t.Errorf("VerifyDataContext = %v, %v, want nil, %v", res, err, tc.want)
			}

			if calls := rec.snapshot(); len(calls) != 0 {
				t.Errorf("got progress callbacks %v, want none", calls)
			}
		})
	}
}

func TestCreateContextCancelDuringHashing(t *testing.T) {
	prev := progressInterval
	progressInterval = time.Microsecond
	t.Cleanup(func() { progressInterval = prev })

	content := writeCancelContent(t, 64<<20)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var midway atomic.Bool
	rec := progressRecorder{onCall: func(completed, total int) {
		if completed > 0 && completed < total {
			midway.Store(true)
			cancel()
		}
	}}
	opts := cancelCreateOptions(t, content)
	opts.ProgressCallback = rec.callback

	info, err := CreateContext(ctx, opts)
	rec.finish(t)
	if !midway.Load() {
		t.Fatal("create finished before any progress callback; the test did not cancel during hashing")
	}
	if info != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("CreateContext = %v, %v, want nil, context.Canceled", info, err)
	}
	assertNoFile(t, opts.OutputPath)
	for _, c := range rec.snapshot() {
		if c.completed == c.total {
			t.Errorf("got completion callback %v after cancel", c)
		}
	}
}

func TestCreateContextCancelOnLastPiece(t *testing.T) {
	content := writeCancelContent(t, 256<<10)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	rec := progressRecorder{onCall: func(completed, total int) {
		if completed == total {
			cancel()
		}
	}}
	opts := cancelCreateOptions(t, content)
	opts.ProgressCallback = rec.callback

	info, err := CreateContext(ctx, opts)
	rec.finish(t)
	if info != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("CreateContext = %v, %v, want nil, context.Canceled", info, err)
	}
	assertNoFile(t, opts.OutputPath)
}

func TestVerifyDataContextCancelDuringVerify(t *testing.T) {
	prev := progressInterval
	progressInterval = time.Microsecond
	t.Cleanup(func() { progressInterval = prev })

	content := writeCancelContent(t, 64<<20)
	opts := cancelVerifyOptions(t, content)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var midway atomic.Bool
	rec := progressRecorder{onCall: func(completed, total int) {
		if completed < total {
			midway.Store(true)
			cancel()
		}
	}}
	opts.ProgressCallback = rec.callback

	res, err := VerifyDataContext(ctx, opts)
	rec.finish(t)
	if !midway.Load() {
		t.Fatal("verify finished before any progress callback; the test did not cancel during verify")
	}
	if res != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("VerifyDataContext = %v, %v, want nil, context.Canceled", res, err)
	}
	for _, c := range rec.snapshot() {
		if c.completed == c.total {
			t.Errorf("got completion callback %v after cancel", c)
		}
	}
}

func TestContextFunctionsWithBackgroundMatchOldFunctions(t *testing.T) {
	content := writeCancelContent(t, 256<<10)
	ctx := context.Background()

	var oldRec, newRec progressRecorder
	oldOpts := cancelCreateOptions(t, content)
	oldOpts.ProgressCallback = oldRec.callback
	newOpts := cancelCreateOptions(t, content)
	newOpts.ProgressCallback = newRec.callback

	oldInfo, err := Create(oldOpts)
	if err != nil {
		t.Fatal(err)
	}
	newInfo, err := CreateContext(ctx, newOpts)
	if err != nil {
		t.Fatal(err)
	}
	if oldInfo.InfoHash != newInfo.InfoHash {
		t.Errorf("CreateContext info hash = %s, Create = %s", newInfo.InfoHash, oldInfo.InfoHash)
	}
	if !slices.Equal(oldRec.snapshot(), newRec.snapshot()) {
		t.Errorf("CreateContext callbacks = %v, Create = %v", newRec.snapshot(), oldRec.snapshot())
	}

	tr, err := CreateTorrentContext(ctx, cancelCreateOptions(t, content))
	if err != nil {
		t.Fatal(err)
	}
	if got := tr.HashInfoBytes().String(); got != oldInfo.InfoHash {
		t.Errorf("CreateTorrentContext info hash = %s, Create = %s", got, oldInfo.InfoHash)
	}

	var oldVerify, newVerify progressRecorder
	verifyOpts := VerifyOptions{TorrentPath: oldInfo.Path, ContentPath: content, Quiet: true, Workers: 1}
	verifyOpts.ProgressCallback = oldVerify.callback
	oldRes, err := VerifyData(verifyOpts)
	if err != nil {
		t.Fatal(err)
	}
	verifyOpts.ProgressCallback = newVerify.callback
	newRes, err := VerifyDataContext(ctx, verifyOpts)
	if err != nil {
		t.Fatal(err)
	}
	if oldRes.GoodPieces != newRes.GoodPieces || oldRes.TotalPieces != newRes.TotalPieces || oldRes.Completion != newRes.Completion {
		t.Errorf("VerifyDataContext result = %+v, VerifyData = %+v", newRes, oldRes)
	}
	if !slices.Equal(oldVerify.snapshot(), newVerify.snapshot()) {
		t.Errorf("VerifyDataContext callbacks = %v, VerifyData = %v", newVerify.snapshot(), oldVerify.snapshot())
	}
}

func TestWorkersStopBeforeReadingWhenContextDone(t *testing.T) {
	const size, pieceLen = 1 << 20, 32 << 10
	content := writeCancelContent(t, size)
	files := []fileEntry{{path: content, length: size}}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	h := newPieceHasher(files, pieceLen, size/pieceLen, &callbackDisplayer{})
	if err := h.hashPiecesContext(ctx, 2); !errors.Is(err, context.Canceled) {
		t.Errorf("hashPiecesContext error = %v, want context.Canceled", err)
	}
	if n := h.bytesProcessed.Load(); n != 0 {
		t.Errorf("hasher read %d bytes after cancel, want 0", n)
	}

	display := NewDisplay(NewFormatter(false))
	display.SetQuiet(true)
	v := &pieceVerifier{
		torrentInfo: &metainfo.Info{PieceLength: pieceLen, Pieces: make([]byte, 20*(size/pieceLen))},
		display:     display,
		files:       files,
		pieceLen:    pieceLen,
		numPieces:   size / pieceLen,
	}
	if err := v.verifyPiecesContext(ctx, 2); !errors.Is(err, context.Canceled) {
		t.Errorf("verifyPiecesContext error = %v, want context.Canceled", err)
	}
	if n := v.bytesVerified.Load(); n != 0 {
		t.Errorf("verifier read %d bytes after cancel, want 0", n)
	}
}

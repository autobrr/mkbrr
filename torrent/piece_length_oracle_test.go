// Copyright (c) 2025-2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package torrent

import (
	"fmt"
	"math/bits"
	"slices"
	"testing"

	"github.com/autobrr/mkbrr/internal/trackers"
)

// TestChoosePieceLengthMatchesOracle compares choosePieceLength with a copy of
// the piece length choice before the bounds moved into pieceLengthBounds. A
// difference for a setting that both accept changes the info hash.
func TestChoosePieceLengthMatchesOracle(t *testing.T) {
	ruleSets := []trackers.Rules{{}}
	for _, r := range trackers.Configs() {
		noLimit, smallLimit := r, r
		noLimit.MaxTorrentSize = 0
		smallLimit.MaxTorrentSize = 64 << 10
		ruleSets = append(ruleSets, r, noLimit, smallLimit)
	}

	var sizes []int64
	for exp := 10; exp <= 42; exp++ {
		sizes = append(sizes, int64(1)<<exp, int64(3)<<(exp-1))
	}

	exps := []*uint{nil}
	for e := uint(10); e <= 28; e++ {
		exps = append(exps, new(e))
	}
	targets := []*uint{nil}
	for _, n := range []uint{1, 1000, 50000} {
		targets = append(targets, new(n))
	}

	// The fake size grows with the piece count, so the size limit can apply.
	torrentSize := func(totalSize int64) func(uint) (uint64, error) {
		return func(exp uint) (uint64, error) {
			pieces, err := pieceCountForSize(totalSize, int64(1)<<exp)
			return uint64(pieces)*20 + 1024, err
		}
	}

	cases := 0
	for _, rules := range ruleSets {
		var urls []string
		if len(rules.URLs) > 0 {
			urls = rules.URLs[:1]
		}
		for _, size := range sizes {
			check := func(opts CreateOptions) {
				t.Helper()
				cases++
				opts.TrackerURLs = urls
				got, gotNotices, gotErr := choosePieceLength(size, opts, rules, torrentSize(size))
				want, wantNotices, wantErr := oracleChoosePieceLength(size, opts, rules, torrentSize(size))
				if gotErr == nil && wantErr != nil && widenedNoTrackerLowerBound(opts, rules) {
					return
				}
				if (gotErr != nil) != (wantErr != nil) || got != want || !slices.Equal(gotNotices, wantNotices) {
					t.Fatalf("rules %v (limit %d), size %d, -l %v, -m %v, target %v: got %d %v %v, want %d %v %v",
						urls, rules.MaxTorrentSize, size, deref(opts.PieceLengthExp), deref(opts.MaxPieceLength), deref(opts.TargetPieceCount),
						got, gotNotices, gotErr, want, wantNotices, wantErr)
				}
			}
			for _, m := range exps {
				for _, target := range targets {
					check(CreateOptions{MaxPieceLength: m, TargetPieceCount: target})
				}
				if m != nil {
					// -l skips the -m check, so -m reaches only the size limit retry
					for _, userMax := range exps {
						check(CreateOptions{PieceLengthExp: m, MaxPieceLength: userMax})
					}
				}
			}
		}
	}
	t.Logf("%d cases", cases)
}

// widenedNoTrackerLowerBound reports whether opts uses the 32 KiB exponent that
// is now accepted with no tracker rules, where the oracle still rejects it.
// The oracle has no info hash to compare for these settings.
func widenedNoTrackerLowerBound(opts CreateOptions, rules trackers.Rules) bool {
	if len(rules.PieceSizeRanges) > 0 || rules.UseDefaultRanges || opts.TargetPieceCount != nil {
		return false
	}
	is15 := func(p *uint) bool { return p != nil && *p == 15 }
	if opts.PieceLengthExp != nil {
		return is15(opts.PieceLengthExp)
	}
	return is15(opts.MaxPieceLength)
}

func deref(p *uint) string {
	if p == nil {
		return "nil"
	}
	return fmt.Sprint(*p)
}

// The functions below are copies of the piece length choice from develop at
// 85f1a79. Do not change them.

func oracleTrackerPieceLengthBounds(rules trackers.Rules, maxPieceLength *uint) (uint, uint) {
	minExp := uint(16)
	maxExp := uint(24)

	if len(rules.PieceSizeRanges) > 0 {
		minExp = 14
		maxExp = 27
	}

	if rules.MaxPieceLength > 0 {
		maxExp = rules.MaxPieceLength
	}

	if maxPieceLength != nil {
		maxExp = min(maxExp, *maxPieceLength, 27)
	}

	return minExp, max(maxExp, minExp)
}

func oracleSizeLimitPieceLengthCeiling(rules trackers.Rules, maxPieceLength *uint) uint {
	if rules.MaxPieceLength > 0 {
		if maxPieceLength != nil {
			return min(*maxPieceLength, rules.MaxPieceLength)
		}
		return rules.MaxPieceLength
	}
	if maxPieceLength != nil {
		return min(*maxPieceLength, 27)
	}
	if len(rules.PieceSizeRanges) > 0 {
		return 27
	}
	return 24
}

func oracleChoosePieceLength(totalSize int64, opts CreateOptions, rules trackers.Rules, torrentSize func(exp uint) (uint64, error)) (uint, []Notice, error) {
	if opts.PieceLengthExp != nil && opts.TargetPieceCount != nil {
		return 0, nil, fmt.Errorf("cannot use both piece length and target piece count; use one or the other")
	}
	if opts.TargetPieceCount != nil && opts.PieceLengthExp == nil && *opts.TargetPieceCount == 0 {
		return 0, nil, fmt.Errorf("target piece count must be greater than zero")
	}

	var (
		exp     uint
		notices []Notice
	)
	maxExp := uint(27)
	if rules.MaxPieceLength > 0 {
		maxExp = rules.MaxPieceLength
	}
	if opts.PieceLengthExp != nil {
		exp = *opts.PieceLengthExp

		minExp, _ := oracleTrackerPieceLengthBounds(rules, nil)
		if exp < minExp || exp > maxExp {
			return 0, nil, fmt.Errorf("piece length exponent out of range")
		}

		if rec, ok := rules.PieceSizeExp(uint64(totalSize)); ok {
			notices = append(notices, Notice{Text: fmt.Sprintf("using tracker-specific range for content size: %d MiB (recommended: %s pieces)",
				totalSize>>20, formatPieceSize(rec))})
			if exp != rec {
				notices = append(notices, Notice{Warn: true, Text: fmt.Sprintf("custom piece length %s differs from recommendation",
					formatPieceSize(exp))})
			}
		}
	} else {
		if opts.MaxPieceLength != nil {
			minExp, _ := oracleTrackerPieceLengthBounds(rules, nil)
			if opts.TargetPieceCount != nil {
				minExp = 16
			}
			if *opts.MaxPieceLength < minExp || *opts.MaxPieceLength > maxExp {
				return 0, nil, fmt.Errorf("max piece length exponent out of range")
			}
		}

		var notice *Notice
		if opts.TargetPieceCount != nil {
			exp, notice = oraclePieceLengthFromTarget(totalSize, *opts.TargetPieceCount, opts.MaxPieceLength, rules)
		} else {
			exp, notice = oracleAutomaticPieceLength(totalSize, opts.MaxPieceLength, rules)
		}
		if notice != nil {
			notices = append(notices, *notice)
		}
	}

	if rules.MaxTorrentSize == 0 || torrentSize == nil {
		return exp, notices, nil
	}

	ceiling := oracleSizeLimitPieceLengthCeiling(rules, opts.MaxPieceLength)
	start := exp
	for {
		size, err := torrentSize(exp)
		if err != nil {
			return 0, nil, err
		}
		if size <= rules.MaxTorrentSize {
			break
		}
		if exp >= ceiling {
			return 0, nil, fmt.Errorf("unable to create torrent under size limit")
		}
		exp++
	}
	if exp != start {
		notices = append(notices, Notice{Warn: true, Text: fmt.Sprintf("raised piece length from %s to %s to fit the %.1f KiB torrent size limit",
			formatPieceSize(start), formatPieceSize(exp), float64(rules.MaxTorrentSize)/(1<<10))})
	}
	return exp, notices, nil
}

func oraclePieceLengthFromTarget(totalSize int64, targetCount uint, maxPieceLength *uint, rules trackers.Rules) (uint, *Notice) {
	minExp := uint(16)
	maxExp := uint(24)

	trackerCap := rules.MaxPieceLength
	if maxPieceLength != nil {
		userMax := min(*maxPieceLength, 27)
		if trackerCap > 0 {
			maxExp = min(userMax, trackerCap)
		} else {
			maxExp = userMax
		}
	} else if trackerCap > 0 {
		maxExp = trackerCap
	}

	maxExp = max(maxExp, minExp)

	ratio := uint64(0)
	if targetCount > 0 && totalSize > 0 {
		ratio = uint64(totalSize) / uint64(targetCount)
	}

	var exp uint
	if ratio == 0 {
		exp = minExp
	} else {
		exp = uint(bits.Len64(ratio)) - 1
	}

	clamped := min(max(exp, minExp), maxExp)
	if clamped == exp {
		return clamped, nil
	}
	actualPieces := (uint64(totalSize) + (1 << clamped) - 1) / (1 << clamped)
	return clamped, &Notice{Text: fmt.Sprintf("target piece count %d adjusted: using %s pieces (%d actual pieces) due to constraints",
		targetCount, formatPieceSize(clamped), actualPieces)}
}

func oracleAutomaticPieceLength(totalSize int64, maxPieceLength *uint, rules trackers.Rules) (uint, *Notice) {
	if exp, ok := rules.PieceSizeExp(uint64(totalSize)); ok {
		minExp, maxExp := oracleTrackerPieceLengthBounds(rules, maxPieceLength)
		exp = min(max(exp, minExp), maxExp)
		return exp, &Notice{Text: fmt.Sprintf("using tracker-specific range for content size: %d MiB (recommended: %s pieces)",
			totalSize>>20, formatPieceSize(exp))}
	}

	maxExp := uint(24)
	if rules.MaxPieceLength > 0 {
		maxExp = rules.MaxPieceLength
	}

	if maxPieceLength != nil {
		maxExp = min(*maxPieceLength, 27)
	}

	size := uint64(max(totalSize, 1))

	var exp uint
	for _, r := range trackers.DefaultPieceSizeRanges {
		if size <= r.MaxSize {
			exp = r.PieceExp
			break
		}
	}

	return min(exp, maxExp), nil
}

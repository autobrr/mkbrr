// Copyright (c) 2025-2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package torrent

import "time"

// progressInterval is a var so that tests can get progress callbacks while small files hash.
var progressInterval = 200 * time.Millisecond

// Displayer defines the interface for displaying progress during torrent creation
type Displayer interface {
	ShowProgress(total int)
	UpdateProgress(completed int, hashrate float64)
	ShowFiles(files []fileEntry, numWorkers int)
	ShowSeasonPackWarnings(info *SeasonPackInfo)
	FinishProgress()
	IsBatch() bool
}

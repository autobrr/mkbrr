package torrent

import "testing"

func TestCallbackDisplayerReportsHashRateInMiB(t *testing.T) {
	var got float64
	displayer := &callbackDisplayer{
		callback: func(_, _ int, hashRate float64) {
			got = hashRate
		},
	}

	displayer.ShowProgress(1)
	displayer.UpdateProgress(1, 1024*1024)

	if got != 1 {
		t.Fatalf("callback hash rate = %v, want 1 MiB/s", got)
	}
}

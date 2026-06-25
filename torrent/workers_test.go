package torrent

import "testing"

func TestAutoWorkerCount(t *testing.T) {
	tests := []struct {
		name               string
		cpuCount           int
		allowOversubscribe bool
		goos               string
		want               int
	}{
		{
			name:               "darwin keeps cpu count for large workloads",
			cpuCount:           12,
			allowOversubscribe: true,
			goos:               "darwin",
			want:               12,
		},
		{
			name:               "linux oversubscribes for large workloads",
			cpuCount:           12,
			allowOversubscribe: true,
			goos:               "linux",
			want:               24,
		},
		{
			name:               "small workload stays at cpu count",
			cpuCount:           12,
			allowOversubscribe: false,
			goos:               "linux",
			want:               12,
		},
		{
			name:               "zero cpu count stays zero",
			cpuCount:           0,
			allowOversubscribe: true,
			goos:               "linux",
			want:               0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := autoWorkerCount(tt.cpuCount, tt.allowOversubscribe, tt.goos)
			if got != tt.want {
				t.Fatalf("autoWorkerCount(%d, %t, %q) = %d, want %d", tt.cpuCount, tt.allowOversubscribe, tt.goos, got, tt.want)
			}
		})
	}
}

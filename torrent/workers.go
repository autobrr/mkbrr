package torrent

import "runtime"

// autoWorkerCount returns the number of hashing worker goroutines to use.
//
// On non-darwin platforms, large workloads oversubscribe to 2x the CPU count.
// Hashing is IO-bound on real storage, and on HDD arrays and FUSE/mergerfs
// mounts keeping more reads in flight than there are cores measurably improves
// wall time (the extra readers hide per-read latency); benchmarking on a real
// 12-core mergerfs box showed ~20% faster wall time at 2x vs 1x workers. The
// extra concurrency costs some kernel/sys CPU, but wall time is what users
// wait on, so the trade is worthwhile on those filesystems. darwin schedules
// IO well enough that oversubscription gives no benefit there.
func autoWorkerCount(cpuCount int, allowOversubscribe bool, goos string) int {
	if cpuCount <= 0 {
		return 0
	}
	if allowOversubscribe && goos != "darwin" {
		return cpuCount * 2
	}
	return cpuCount
}

func defaultWorkerCount(allowOversubscribe bool) int {
	return autoWorkerCount(runtime.NumCPU(), allowOversubscribe, runtime.GOOS)
}

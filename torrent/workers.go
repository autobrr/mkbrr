package torrent

import "runtime"

// autoWorkerCount returns the number of hashing worker goroutines to use.
//
// It never oversubscribes the CPU count. Earlier versions doubled the worker
// count on non-darwin platforms for large workloads, but measurement on real
// IO-bound storage (HDD arrays, FUSE/mergerfs) showed that extra read()
// workers add no throughput — wall time is disk-bound — while each additional
// concurrent reader inflates kernel/sys CPU (syscall + page-cache contention).
// Oversubscription was therefore pure waste in every regime measured: on
// CPU-bound (cached) workloads it only adds scheduler and memory pressure, and
// on IO-bound workloads it burns sys CPU for no wall-time gain. The mmap read
// path keeps per-worker syscall overhead low independently of this count.
//
// allowOversubscribe is retained for call-site compatibility but no longer
// changes the result.
func autoWorkerCount(cpuCount int, allowOversubscribe bool, goos string) int {
	if cpuCount <= 0 {
		return 0
	}
	return cpuCount
}

func defaultWorkerCount(allowOversubscribe bool) int {
	return autoWorkerCount(runtime.NumCPU(), allowOversubscribe, runtime.GOOS)
}

package torrent

import "runtime"

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

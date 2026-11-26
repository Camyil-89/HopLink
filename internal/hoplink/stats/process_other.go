//go:build !linux

package stats

type processSample struct {
	rssKB    uint64
	cpuUsecs uint64
}

func processMetricsSupported() bool { return false }

func readProcessSample() (processSample, bool) {
	return processSample{}, false
}
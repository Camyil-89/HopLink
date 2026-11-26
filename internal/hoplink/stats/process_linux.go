//go:build linux

package stats

import (
	"bufio"
	"bytes"
	"os"
	"strconv"
	"strings"
	"syscall"
)

type processSample struct {
	rssKB    uint64
	cpuUsecs uint64
}

func processMetricsSupported() bool { return true }

func readProcessSample() (processSample, bool) {
	rss, ok := readRssKB()
	cpu, okCPU := readCPUUsecs()
	if !ok && !okCPU {
		return processSample{}, false
	}
	return processSample{rssKB: rss, cpuUsecs: cpu}, true
}

func readRssKB() (uint64, bool) {
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return 0, false
	}
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "VmRSS:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			break
		}
		kb, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return 0, false
		}
		return kb, true
	}
	return 0, false
}

func readCPUUsecs() (uint64, bool) {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0, false
	}
	usec := uint64(ru.Utime.Sec)*1_000_000 + uint64(ru.Utime.Usec)
	usec += uint64(ru.Stime.Sec)*1_000_000 + uint64(ru.Stime.Usec)
	return usec, true
}
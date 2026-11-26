package stats

import (
	"math"
	"time"
)

// ProcessRssDay is per-day RSS aggregates (kilobytes).
type ProcessRssDay struct {
	Last uint64 `json:"last"`
	Max  uint64 `json:"max"`
}

// ProcessCpuDay is per-day CPU aggregates (percent of one core).
type ProcessCpuDay struct {
	Avg float64 `json:"avg"`
	Max float64 `json:"max"`
}

// ProcessDay is calendar-day process resource usage.
type ProcessDay struct {
	Date   string          `json:"date"`
	RssKB  ProcessRssDay   `json:"rssKB"`
	CpuPct ProcessCpuDay   `json:"cpuPct"`
}

// ProcessCurrent is the latest sample at flush time.
type ProcessCurrent struct {
	RssKB  uint64  `json:"rssKB"`
	CpuPct float64 `json:"cpuPct"`
}

// ProcessStats is hoplink process RAM/CPU history in stats.json.
type ProcessStats struct {
	SampledAt         string         `json:"sampledAt,omitempty"`
	SampleIntervalSec int            `json:"sampleIntervalSec,omitempty"`
	Current           ProcessCurrent `json:"current"`
	Days              []ProcessDay   `json:"days"`
}

func roundPct(v float64) float64 {
	if v < 0 {
		return 0
	}
	return math.Round(v*10) / 10
}

func appendProcessDay(days []ProcessDay, d ProcessDay) []ProcessDay {
	for i := range days {
		if days[i].Date == d.Date {
			if d.RssKB.Last > days[i].RssKB.Last {
				days[i].RssKB.Last = d.RssKB.Last
			}
			if d.RssKB.Max > days[i].RssKB.Max {
				days[i].RssKB.Max = d.RssKB.Max
			}
			if d.CpuPct.Max > days[i].CpuPct.Max {
				days[i].CpuPct.Max = d.CpuPct.Max
			}
			if d.CpuPct.Avg > 0 {
				days[i].CpuPct.Avg = d.CpuPct.Avg
			}
			return days
		}
	}
	return append(days, d)
}

func (c *Collector) loadProcess(snap *File, today string) {
	if snap.Process == nil {
		return
	}
	for _, d := range snap.Process.Days {
		if d.Date == today {
			c.procDayRssLast = d.RssKB.Last
			c.procDayRssMax = d.RssKB.Max
			if d.CpuPct.Max > c.procCpuMax {
				c.procCpuMax = d.CpuPct.Max
			}
			continue
		}
		c.procArchived = append(c.procArchived, d)
	}
	c.procCurrentRss = snap.Process.Current.RssKB
	c.procCurrentCPU = snap.Process.Current.CpuPct
}

func (c *Collector) sampleProcessLocked() {
	sample, ok := readProcessSample()
	if !ok {
		return
	}
	now := time.Now()
	first := c.procLastWall.IsZero()
	cpuPct := 0.0
	if !first && sample.cpuUsecs >= c.procLastCPU {
		wall := now.Sub(c.procLastWall).Seconds()
		if wall > 0 {
			delta := float64(sample.cpuUsecs - c.procLastCPU)
			cpuPct = (delta / (wall * 1e6)) * 100
		}
	}
	c.procLastWall = now
	c.procLastCPU = sample.cpuUsecs

	if sample.rssKB > 0 {
		// First sample after load only seeds CPU baseline; keep persisted current RSS.
		if !(first && c.procCurrentRss > 0) {
			c.procDayRssLast = sample.rssKB
			if sample.rssKB > c.procDayRssMax {
				c.procDayRssMax = sample.rssKB
			}
			c.procCurrentRss = sample.rssKB
		}
	}
	if first {
		return
	}
	c.procCurrentCPU = roundPct(cpuPct)
	c.procCpuSum += cpuPct
	c.procCpuN++
	if cpuPct > c.procCpuMax {
		c.procCpuMax = cpuPct
	}
}

func (c *Collector) archiveProcessDay(date string) {
	if c.procDayRssLast == 0 && c.procCpuN == 0 {
		return
	}
	avg := 0.0
	if c.procCpuN > 0 {
		avg = c.procCpuSum / float64(c.procCpuN)
	}
	c.procArchived = appendProcessDay(c.procArchived, ProcessDay{
		Date: date,
		RssKB: ProcessRssDay{
			Last: c.procDayRssLast,
			Max:  c.procDayRssMax,
		},
		CpuPct: ProcessCpuDay{
			Avg: roundPct(avg),
			Max: roundPct(c.procCpuMax),
		},
	})
	c.procDayRssLast = 0
	c.procDayRssMax = 0
	c.procCpuSum = 0
	c.procCpuN = 0
	c.procCpuMax = 0
}

func (c *Collector) processSnapshotLocked(today string) *ProcessStats {
	if !processMetricsSupported() {
		return nil
	}
	days := make([]ProcessDay, len(c.procArchived))
	copy(days, c.procArchived)
	if c.procDayRssLast > 0 || c.procCpuN > 0 {
		avg := 0.0
		if c.procCpuN > 0 {
			avg = c.procCpuSum / float64(c.procCpuN)
		}
		days = appendProcessDay(days, ProcessDay{
			Date: today,
			RssKB: ProcessRssDay{
				Last: c.procDayRssLast,
				Max:  c.procDayRssMax,
			},
			CpuPct: ProcessCpuDay{
				Avg: roundPct(avg),
				Max: roundPct(c.procCpuMax),
			},
		})
	}
	if len(days) == 0 && c.procCurrentRss == 0 && c.procCurrentCPU == 0 {
		return nil
	}
	return &ProcessStats{
		SampledAt:         time.Now().UTC().Format(time.RFC3339),
		SampleIntervalSec: int(flushInterval / time.Second),
		Current: ProcessCurrent{
			RssKB:  c.procCurrentRss,
			CpuPct: roundPct(c.procCurrentCPU),
		},
		Days: days,
	}
}
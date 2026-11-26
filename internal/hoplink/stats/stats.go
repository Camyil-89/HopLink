package stats

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"hoplink/internal/hoplink/log"

	hyserver "github.com/apernet/hysteria/core/v2/server"
)

// Default is the process-wide collector when -stats-file is set.
var Default *Collector

const flushInterval = 30 * time.Second

// Volume is traffic in kilobytes (rounded down when persisted).
type Volume struct {
	In  uint64 `json:"in"`
	Out uint64 `json:"out"`
}

// DayStat is per-calendar-day traffic in kilobytes.
type DayStat struct {
	Date string `json:"date"`
	In   uint64 `json:"in"`
	Out  uint64 `json:"out"`
}

// UserStat is cumulative and daily traffic for one authenticated client id.
type UserStat struct {
	ID   string    `json:"id"`
	In   uint64    `json:"in"`
	Out  uint64    `json:"out"`
	Days []DayStat `json:"days"`
}

// File is the on-disk JSON shape.
type File struct {
	Total   Volume        `json:"total"`
	Days    []DayStat     `json:"days"`
	Users   []UserStat    `json:"users"`
	Process *ProcessStats `json:"process,omitempty"`
}

// Collector counts traffic with atomics and periodically flushes to disk.
type Collector struct {
	path string

	totalIn  atomic.Uint64
	totalOut atomic.Uint64
	dayIn    atomic.Uint64
	dayOut   atomic.Uint64

	mu          sync.Mutex
	archivedDay string
	archived    []DayStat
	users       sync.Map // string -> *userCounters
	userArchive map[string][]DayStat

	procArchived   []ProcessDay
	procDayRssLast uint64
	procDayRssMax  uint64
	procCpuSum     float64
	procCpuN       int
	procCpuMax     float64
	procCurrentRss uint64
	procCurrentCPU float64
	procLastCPU    uint64
	procLastWall   time.Time

	stop context.CancelFunc
}

type userCounters struct {
	totalIn  atomic.Uint64
	totalOut atomic.Uint64
	dayIn    atomic.Uint64
	dayOut   atomic.Uint64
}

// Start loads or creates statistics at path and begins the flush loop.
func Start(path string) (*Collector, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		path = "stats.json"
	}
	if !filepath.IsAbs(path) {
		if wd, err := os.Getwd(); err == nil && wd != "" {
			path = filepath.Join(wd, path)
		}
	}
	path = filepath.Clean(path)
	c := &Collector{path: path, userArchive: make(map[string][]DayStat)}
	if err := c.load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.stop = cancel
	Default = c
	go c.loop(ctx)
	if err := c.persist(); err != nil {
		log.Warnf("[STATS_PERSIST] path=%s err=%v", path, err)
	}
	return c, nil
}

// Close stops the flush loop and writes a final snapshot.
func (c *Collector) Close() {
	if c == nil {
		return
	}
	if c.stop != nil {
		c.stop()
	}
	_ = c.persist()
}

func bytesToKB(b uint64) uint64 {
	return b / 1024
}

func kbToBytes(kb uint64) uint64 {
	return kb * 1024
}

func (c *Collector) load() error {
	data, err := os.ReadFile(c.path)
	if err != nil {
		return err
	}
	var snap File
	if err := json.Unmarshal(data, &snap); err != nil {
		return fmt.Errorf("parse stats: %w", err)
	}
	c.totalIn.Store(kbToBytes(snap.Total.In))
	c.totalOut.Store(kbToBytes(snap.Total.Out))
	today := time.Now().Format("2006-01-02")
	for _, d := range snap.Days {
		if d.Date == today {
			c.dayIn.Store(kbToBytes(d.In))
			c.dayOut.Store(kbToBytes(d.Out))
			continue
		}
		c.archived = append(c.archived, d)
	}
	if len(snap.Days) > 0 {
		last := snap.Days[len(snap.Days)-1].Date
		if last == today {
			c.archivedDay = today
		}
	}
	for _, u := range snap.Users {
		uc := &userCounters{}
		uc.totalIn.Store(kbToBytes(u.In))
		uc.totalOut.Store(kbToBytes(u.Out))
		var past []DayStat
		for _, d := range u.Days {
			if d.Date == today {
				uc.dayIn.Store(kbToBytes(d.In))
				uc.dayOut.Store(kbToBytes(d.Out))
				continue
			}
			past = append(past, d)
		}
		if len(past) > 0 {
			c.userArchive[u.ID] = past
		}
		c.users.Store(u.ID, uc)
	}
	c.loadProcess(&snap, time.Now().Format("2006-01-02"))
	return nil
}

func (c *Collector) loop(ctx context.Context) {
	tick := time.NewTicker(flushInterval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			if err := c.persist(); err != nil {
				log.Warnf("[STATS_PERSIST] path=%s err=%v", c.path, err)
			}
		}
	}
}

func (c *Collector) maybeRotateDay(today string) {
	if c.archivedDay == today {
		return
	}
	if c.archivedDay != "" {
		c.archiveDay(c.archivedDay, c.dayIn.Swap(0), c.dayOut.Swap(0))
		c.users.Range(func(key, value any) bool {
			id := key.(string)
			uc := value.(*userCounters)
			in := uc.dayIn.Swap(0)
			out := uc.dayOut.Swap(0)
			if in == 0 && out == 0 {
				return true
			}
			c.userArchive[id] = append(c.userArchive[id], DayStat{
				Date: c.archivedDay,
				In:   bytesToKB(in),
				Out:  bytesToKB(out),
			})
			return true
		})
		c.archiveProcessDay(c.archivedDay)
	}
	c.archivedDay = today
}

func (c *Collector) archiveDay(date string, inBytes, outBytes uint64) {
	if inBytes == 0 && outBytes == 0 {
		return
	}
	c.archived = appendDay(c.archived, DayStat{
		Date: date,
		In:   bytesToKB(inBytes),
		Out:  bytesToKB(outBytes),
	})
}

func appendDay(days []DayStat, d DayStat) []DayStat {
	for i := range days {
		if days[i].Date == d.Date {
			days[i].In += d.In
			days[i].Out += d.Out
			return days
		}
	}
	return append(days, d)
}

func (c *Collector) persist() error {
	if c == nil || c.path == "" {
		return nil
	}
	today := time.Now().Format("2006-01-02")
	c.mu.Lock()
	c.sampleProcessLocked()
	c.maybeRotateDay(today)
	snap := c.snapshotLocked(today)
	c.mu.Unlock()
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(c.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	tmp := c.path + ".tmp"
	_ = os.Remove(tmp)
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	if err := os.Rename(tmp, c.path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func (c *Collector) snapshotLocked(today string) File {
	days := make([]DayStat, len(c.archived))
	copy(days, c.archived)
	todayVol := DayStat{
		Date: today,
		In:   bytesToKB(c.dayIn.Load()),
		Out:  bytesToKB(c.dayOut.Load()),
	}
	if todayVol.In > 0 || todayVol.Out > 0 {
		days = appendDay(days, todayVol)
	}
	var users []UserStat
	c.users.Range(func(key, value any) bool {
		id := key.(string)
		uc := value.(*userCounters)
		udays := make([]DayStat, len(c.userArchive[id]))
		copy(udays, c.userArchive[id])
		uToday := DayStat{
			Date: today,
			In:   bytesToKB(uc.dayIn.Load()),
			Out:  bytesToKB(uc.dayOut.Load()),
		}
		if uToday.In > 0 || uToday.Out > 0 {
			udays = appendDay(udays, uToday)
		}
		users = append(users, UserStat{
			ID:   id,
			In:   bytesToKB(uc.totalIn.Load()),
			Out:  bytesToKB(uc.totalOut.Load()),
			Days: udays,
		})
		return true
	})
	return File{
		Total: Volume{
			In:  bytesToKB(c.totalIn.Load()),
			Out: bytesToKB(c.totalOut.Load()),
		},
		Days:    days,
		Users:   users,
		Process: c.processSnapshotLocked(today),
	}
}

func (c *Collector) addBytes(inBytes, outBytes uint64) {
	if c == nil || (inBytes == 0 && outBytes == 0) {
		return
	}
	if inBytes > 0 {
		c.totalIn.Add(inBytes)
		c.dayIn.Add(inBytes)
	}
	if outBytes > 0 {
		c.totalOut.Add(outBytes)
		c.dayOut.Add(outBytes)
	}
}

func (c *Collector) addUser(id string, inBytes, outBytes uint64) {
	if c == nil || id == "" || id == "relay" {
		return
	}
	raw, _ := c.users.LoadOrStore(id, &userCounters{})
	uc := raw.(*userCounters)
	if inBytes > 0 {
		uc.totalIn.Add(inBytes)
		uc.dayIn.Add(inBytes)
	}
	if outBytes > 0 {
		uc.totalOut.Add(outBytes)
		uc.dayOut.Add(outBytes)
	}
}

// AddIn records bytes received by this hop.
func AddIn(n int) {
	if Default == nil || n <= 0 {
		return
	}
	Default.addBytes(uint64(n), 0)
}

// AddOut records bytes sent by this hop.
func AddOut(n int) {
	if Default == nil || n <= 0 {
		return
	}
	Default.addBytes(0, uint64(n))
}

// LogTraffic implements hyserver.TrafficLogger.
// Stored in: in = client download (Rx), out = client upload (Tx).
func (c *Collector) LogTraffic(id string, tx, rx uint64) bool {
	if c == nil {
		return true
	}
	c.addBytes(rx, tx)
	c.addUser(id, rx, tx)
	return true
}

func (c *Collector) LogOnlineState(string, bool) {}

func (c *Collector) TraceStream(hyserver.HyStream, *hyserver.StreamStats) {}

func (c *Collector) UntraceStream(hyserver.HyStream) {}
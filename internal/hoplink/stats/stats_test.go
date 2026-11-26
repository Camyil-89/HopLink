package stats

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCollectorPersistAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stats.json")
	c, err := Start(path)
	if err != nil {
		t.Fatal(err)
	}
	c.LogTraffic("alice", 2048, 4096)
	c.LogTraffic("bob", 1024, 0)
	AddIn(512)
	AddOut(256)
	c.Close()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var snap File
	if err := json.Unmarshal(data, &snap); err != nil {
		t.Fatal(err)
	}
	if snap.Total.In < 4 || snap.Total.Out < 3 {
		t.Fatalf("total: %+v", snap.Total)
	}
	if len(snap.Users) != 2 {
		t.Fatalf("users: %+v", snap.Users)
	}
	var alice *UserStat
	for i := range snap.Users {
		if snap.Users[i].ID == "alice" {
			alice = &snap.Users[i]
			break
		}
	}
	if alice == nil || alice.In < 4 || alice.Out < 2 {
		t.Fatalf("alice stats: %+v", alice)
	}

	Default = nil
	c2, err := Start(path)
	if err != nil {
		t.Fatal(err)
	}
	defer c2.Close()
	if c2.totalIn.Load() < 4096 {
		t.Fatalf("reload total in bytes %d", c2.totalIn.Load())
	}
}

func TestProcessSectionRoundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stats.json")
	snap := File{
		Total: Volume{In: 1, Out: 2},
		Process: &ProcessStats{
			SampleIntervalSec: 30,
			Current:           ProcessCurrent{RssKB: 40960, CpuPct: 1.5},
			Days: []ProcessDay{
				{
					Date:  time.Now().Format("2006-01-02"),
					RssKB: ProcessRssDay{Last: 40000, Max: 42000},
					CpuPct: ProcessCpuDay{Avg: 2.0, Max: 5.0},
				},
			},
		},
	}
	data, err := json.Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	Default = nil
	c, err := Start(path)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if c.procDayRssMax != 42000 {
		t.Fatalf("procDayRssMax=%d", c.procDayRssMax)
	}
	if c.procCurrentRss != 40960 {
		t.Fatalf("procCurrentRss=%d", c.procCurrentRss)
	}
}

func TestDayRotation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stats.json")
	c := &Collector{path: path, userArchive: make(map[string][]DayStat)}
	c.archivedDay = time.Now().Add(-24 * time.Hour).Format("2006-01-02")
	c.dayIn.Store(1024)
	c.dayOut.Store(2048)
	today := time.Now().Format("2006-01-02")
	c.mu.Lock()
	c.maybeRotateDay(today)
	c.mu.Unlock()
	if len(c.archived) != 1 || c.archived[0].In != 1 {
		t.Fatalf("archived: %+v", c.archived)
	}
	if c.dayIn.Load() != 0 {
		t.Fatal("day counters not reset")
	}
}
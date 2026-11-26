package cascade

import (
	"sync"
	"unsafe"
)

// pendingRoutes holds sniff results between RequestHook and Outbound on the entry hop.
// Entries exist only while a stream is connecting; size tracks concurrent dials, not
// client count. Sharding avoids a single map lock under many parallel clients.
// Each shard occupies its own cache line so adjacent locks do not false-share.
type pendingRoutes struct {
	shards [routeShardCount]routeShard

	zeroMu      sync.Mutex
	zeroPending bool
	zeroSet     bool
	_           [cacheLineSize - 16]byte
}

const (
	routeShardCount = 64
	cacheLineSize   = 64
)

type routeShard struct {
	mu    sync.Mutex
	route map[uint64]bool
	_     [cacheLineSize - unsafe.Sizeof(sync.Mutex{}) - unsafe.Sizeof(map[uint64]bool(nil))]byte
}

func newPendingRoutes() pendingRoutes {
	var p pendingRoutes
	for i := range p.shards {
		p.shards[i].route = make(map[uint64]bool)
	}
	return p
}

func (p *pendingRoutes) shardKey(key uint64) *routeShard {
	return &p.shards[key%routeShardCount]
}

func (p *pendingRoutes) store(goroutineID uint64, local bool) {
	if goroutineID == 0 {
		p.zeroMu.Lock()
		p.zeroPending = local
		p.zeroSet = true
		p.zeroMu.Unlock()
		return
	}
	sh := p.shardKey(goroutineID)
	sh.mu.Lock()
	sh.route[goroutineID] = local
	sh.mu.Unlock()
}

func (p *pendingRoutes) take(goroutineID uint64) (bool, bool) {
	if goroutineID == 0 {
		p.zeroMu.Lock()
		defer p.zeroMu.Unlock()
		if !p.zeroSet {
			return false, false
		}
		local := p.zeroPending
		p.zeroSet = false
		return local, true
	}
	sh := p.shardKey(goroutineID)
	sh.mu.Lock()
	local, ok := sh.route[goroutineID]
	if ok {
		delete(sh.route, goroutineID)
	}
	sh.mu.Unlock()
	return local, ok
}
package cascade

import "sync"

// pendingRoutes holds sniff results between RequestHook and Outbound on the entry hop.
// Entries exist only while a stream is connecting; size tracks concurrent dials, not
// client count. Sharding avoids a single map lock under many parallel clients.
type pendingRoutes struct {
	shards [routeShardCount]routeShard

	zeroMu      sync.Mutex
	zeroPending bool
	zeroSet     bool
}

const routeShardCount = 64

type routeShard struct {
	mu    sync.Mutex
	route map[uint64]bool
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
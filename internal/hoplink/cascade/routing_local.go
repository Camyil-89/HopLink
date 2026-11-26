package cascade

import (
	"runtime"
	"sync"
)

const goroutinePrefix = "goroutine "

var gidBufPool = sync.Pool{
	New: func() any {
		b := make([]byte, 64)
		return &b
	},
}

// parseCurrentGoroutineID reads the monotonic goroutine id from the runtime stack line.
// IDs are not reused in practice; the same id is used for paired hook→dial on one stream.
func parseCurrentGoroutineID() uint64 {
	bp := gidBufPool.Get().(*[]byte)
	buf := *bp
	for {
		n := runtime.Stack(buf, false)
		if n < len(buf) {
			id := gidFromStack(buf[:n])
			*bp = buf
			gidBufPool.Put(bp)
			return id
		}
		buf = make([]byte, len(buf)*2)
	}
}

func gidFromStack(line []byte) uint64 {
	if len(line) <= len(goroutinePrefix) {
		return 0
	}
	for i := 0; i < len(goroutinePrefix); i++ {
		if line[i] != goroutinePrefix[i] {
			return 0
		}
	}
	var id uint64
	i := len(goroutinePrefix)
	if line[i] < '0' || line[i] > '9' {
		return 0
	}
	for ; i < len(line); i++ {
		c := line[i]
		if c < '0' || c > '9' {
			return id
		}
		id = id*10 + uint64(c-'0')
	}
	return id
}

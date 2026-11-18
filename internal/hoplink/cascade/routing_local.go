package cascade

import (
	"bytes"
	"runtime"
	"strconv"
)

// parseCurrentGoroutineID reads the monotonic goroutine id from the runtime stack line.
// IDs are not reused in practice; the same id is used for paired hook→dial on one stream.
func parseCurrentGoroutineID() uint64 {
	const prefix = "goroutine "
	for size := 64; size <= 2048; size *= 2 {
		buf := make([]byte, size)
		n := runtime.Stack(buf, false)
		if n == size {
			continue
		}
		line := buf[:n]
		if !bytes.HasPrefix(line, []byte(prefix)) {
			return 0
		}
		line = line[len(prefix):]
		end := 0
		for end < len(line) && line[end] >= '0' && line[end] <= '9' {
			end++
		}
		if end == 0 {
			return 0
		}
		id, err := strconv.ParseUint(string(line[:end]), 10, 64)
		if err != nil {
			return 0
		}
		return id
	}
	return 0
}
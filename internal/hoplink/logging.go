package hoplink

import (
	"fmt"
	"log"
	"strings"
	"sync/atomic"
)

const (
	levelTrace int32 = -2
	levelDebug int32 = -1
	levelInfo  int32 = 0
	levelWarn  int32 = 1
	levelError int32 = 2
)

var currentLogLevel atomic.Int32 // zero means info, including before flag parsing

func setLogLevel(name string) error {
	var level int32
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "trace":
		level = levelTrace
	case "debug":
		level = levelDebug
	case "info":
		level = levelInfo
	case "warn":
		level = levelWarn
	case "error":
		level = levelError
	default:
		return fmt.Errorf("неверный -log-level %q: допустимы trace, debug, info, warn, error", name)
	}
	currentLogLevel.Store(level)
	return nil
}

func logAt(level int32, format string, args ...any) {
	if level < currentLogLevel.Load() {
		return
	}
	label := "INFO"
	switch level {
	case levelTrace:
		label = "TRACE"
	case levelDebug:
		label = "DEBUG"
	case levelWarn:
		label = "WARN"
	case levelError:
		label = "ERROR"
	}
	log.Printf("["+label+"] "+format, args...)
}

func logDebugf(format string, args ...any) { logAt(levelDebug, format, args...) }
func logTracef(format string, args ...any) { logAt(levelTrace, format, args...) }
func logInfof(format string, args ...any)  { logAt(levelInfo, format, args...) }
func logWarnf(format string, args ...any)  { logAt(levelWarn, format, args...) }
func logErrorf(format string, args ...any) { logAt(levelError, format, args...) }

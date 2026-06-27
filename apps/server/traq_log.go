package main

import (
	"fmt"
	"os"
	"sync"
)

const (
	logColorReset  = "\033[0m"
	logColorBlue   = "\033[34m"
	logColorCyan   = "\033[36m"
	logColorGreen  = "\033[32m"
	logColorYellow = "\033[33m"
	logColorRed    = "\033[31m"
)

var traqLogMu sync.Mutex

func traqLog(color string, label string, format string, args ...any) {
	traqLogMu.Lock()
	defer traqLogMu.Unlock()
	_, _ = fmt.Fprintf(os.Stdout, "%s[traQ:%s]%s %s\n", color, label, logColorReset, fmt.Sprintf(format, args...))
}

func traqLogWS(format string, args ...any) {
	traqLog(logColorCyan, "ws", format, args...)
}

func traqLogAPI(format string, args ...any) {
	traqLog(logColorBlue, "api", format, args...)
}

func traqLogOK(format string, args ...any) {
	traqLog(logColorGreen, "ok", format, args...)
}

func traqLogWarn(format string, args ...any) {
	traqLog(logColorYellow, "warn", format, args...)
}

func traqLogError(format string, args ...any) {
	traqLog(logColorRed, "error", format, args...)
}

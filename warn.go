package main

import (
	"fmt"
	"log"
	"sync"
)

type warningCollector struct {
	mu       sync.Mutex
	warnings []string
}

func (w *warningCollector) warn(s string) {
	w.mu.Lock()
	w.warnings = append(w.warnings, s)
	w.mu.Unlock()
	log.Printf("WARNING: %s", s)
}

func (w *warningCollector) Warnf(format string, a ...interface{}) {
	w.warn(fmt.Sprintf(format, a...))
}

func (w *warningCollector) Warn(a ...interface{}) {
	w.warn(fmt.Sprint(a...))
}

func (w *warningCollector) Warns() []string {
	var l []string
	w.mu.Lock()
	l = append(l, w.warnings...)
	w.mu.Unlock()
	return l
}

// WarningCollector is the default warning collector
var WarningCollector = &warningCollector{}

// Warnf logs a warning
func Warnf(format string, a ...interface{}) {
	WarningCollector.Warnf(format, a...)
}

// Warns returns the logged warnings
func Warns() []string {
	return WarningCollector.Warns()
}

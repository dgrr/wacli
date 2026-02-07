package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/term"
)

func isTTY() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

func parseTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("time is required")
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), nil
	}
	// Full datetime: YYYY-MM-DD HH:MM:SS (UTC)
	if t, err := time.ParseInLocation("2006-01-02 15:04:05", s, time.UTC); err == nil {
		return t, nil
	}
	// Date only: YYYY-MM-DD (UTC, midnight)
	if t, err := time.ParseInLocation("2006-01-02", s, time.UTC); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("unsupported time format %q (use RFC3339, YYYY-MM-DD HH:MM:SS, or YYYY-MM-DD; all times UTC)", s)
}

func truncate(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	if max <= 1 {
		return s[:max]
	}
	return s[:max-1] + "…"
}

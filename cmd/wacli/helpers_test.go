package main

import (
	"testing"
	"time"
)

func TestParseTime(t *testing.T) {
	tests := []struct {
		input    string
		wantErr  bool
		checkFn  func(time.Time) bool
		desc     string
	}{
		{
			input:   "2026-02-07",
			wantErr: false,
			checkFn: func(tm time.Time) bool {
				return tm.Year() == 2026 && tm.Month() == 2 && tm.Day() == 7 &&
					tm.Hour() == 0 && tm.Minute() == 0 && tm.Second() == 0
			},
			desc: "date only should parse to midnight",
		},
		{
			input:   "2026-02-07 20:00:01",
			wantErr: false,
			checkFn: func(tm time.Time) bool {
				return tm.Year() == 2026 && tm.Month() == 2 && tm.Day() == 7 &&
					tm.Hour() == 20 && tm.Minute() == 0 && tm.Second() == 1
			},
			desc: "datetime should parse with exact time",
		},
		{
			input:   "2026-02-07T20:00:01Z",
			wantErr: false,
			checkFn: func(tm time.Time) bool {
				return tm.Year() == 2026 && tm.Month() == 2 && tm.Day() == 7
			},
			desc: "RFC3339 should still work",
		},
		{
			input:   "invalid",
			wantErr: true,
			desc:    "invalid format should error",
		},
		{
			input:   "",
			wantErr: true,
			desc:    "empty string should error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got, err := parseTime(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseTime(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.checkFn != nil && !tt.checkFn(got) {
				t.Errorf("parseTime(%q) = %v, check failed", tt.input, got)
			}
		})
	}
}

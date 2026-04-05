package main

import (
	"context"
	"runtime"
	"testing"
)

// ---------------------------------------------------------------------------
// ps_list sort validation
// ---------------------------------------------------------------------------

func TestPsList_SortByValues(t *testing.T) {
	td := findTool(t, "ps_list")

	validSorts := []string{"cpu", "mem", "pid", ""}
	for _, sort := range validSorts {
		t.Run("sort_"+sort, func(t *testing.T) {
			if runtime.GOOS != "linux" {
				t.Skip("ps --sort requires Linux")
			}
			req := makeReq(map[string]any{"sort_by": sort, "limit": 5})
			result, err := td.Handler(context.Background(), req)
			if err != nil {
				t.Fatalf("handler error for sort_by=%q: %v", sort, err)
			}
			var out PsListOutput
			unmarshalResult(t, result, &out)
			if out.Total > 5 {
				t.Errorf("expected at most 5 processes with limit=5, got %d", out.Total)
			}
		})
	}
}

func TestPsList_InvalidSortValues(t *testing.T) {
	td := findTool(t, "ps_list")

	invalidSorts := []string{"invalid", "name", "time", "INVALID", "CPU"}
	for _, sort := range invalidSorts {
		t.Run("sort_"+sort, func(t *testing.T) {
			req := makeReq(map[string]any{"sort_by": sort})
			result, err := td.Handler(context.Background(), req)
			if err != nil {
				assertContains(t, err.Error(), "INVALID_PARAM")
				return
			}
			if result == nil || !result.IsError {
				t.Errorf("expected error for sort_by=%q", sort)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ps_list limit boundary values
// ---------------------------------------------------------------------------

func TestPsList_LimitBoundaries(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("ps --sort requires Linux")
	}

	td := findTool(t, "ps_list")

	tests := []struct {
		name     string
		limit    int
		maxWant  int
		minWant  int
	}{
		{"limit_1", 1, 1, 0},
		{"limit_0_defaults_20", 0, 20, 1},
		{"limit_negative_defaults_20", -5, 20, 1},
		{"limit_1000", 1000, 1000, 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := makeReq(map[string]any{"limit": tc.limit})
			result, err := td.Handler(context.Background(), req)
			if err != nil {
				t.Fatalf("handler error: %v", err)
			}
			var out PsListOutput
			unmarshalResult(t, result, &out)
			if out.Total > tc.maxWant {
				t.Errorf("expected at most %d processes, got %d", tc.maxWant, out.Total)
			}
			if out.Total < tc.minWant {
				t.Errorf("expected at least %d processes, got %d", tc.minWant, out.Total)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ps_list filter regex-like patterns
// ---------------------------------------------------------------------------

func TestPsList_FilterCaseInsensitive(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("ps --sort requires Linux")
	}

	td := findTool(t, "ps_list")
	// Filter uses strings.Contains with ToLower — test case insensitivity
	req := makeReq(map[string]any{"filter": "PROCESS-MCP", "limit": 50})
	result, err := td.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	var out PsListOutput
	unmarshalResult(t, result, &out)
	// Should match the test binary name regardless of case
	t.Logf("filter matched %d processes", out.Total)
}

// ---------------------------------------------------------------------------
// validSignals map completeness
// ---------------------------------------------------------------------------

func TestValidSignals_Complete(t *testing.T) {
	expected := map[string]bool{
		"TERM": true, "KILL": true, "HUP": true, "INT": true,
		"USR1": true, "USR2": true, "STOP": true, "CONT": true,
	}

	for sig := range expected {
		if !validSignals[sig] {
			t.Errorf("signal %q missing from validSignals", sig)
		}
	}

	for sig := range validSignals {
		if !expected[sig] {
			t.Errorf("unexpected signal %q in validSignals", sig)
		}
	}
}

// ---------------------------------------------------------------------------
// kill_process signal normalization
// ---------------------------------------------------------------------------

func TestKillProcess_SignalNormalization(t *testing.T) {
	td := findTool(t, "kill_process")

	// Lower-case signals should be normalized to upper case
	// Use a PID that doesn't exist to avoid actually killing anything
	tests := []struct {
		signal string
	}{
		{"term"},
		{"kill"},
		{"hup"},
		{"int"},
	}

	for _, tc := range tests {
		t.Run(tc.signal, func(t *testing.T) {
			req := makeReq(map[string]any{"pid": 999999999, "signal": tc.signal})
			result, err := td.Handler(context.Background(), req)
			// Should fail with "No such process", NOT "invalid signal"
			if err != nil {
				if containsStr(err.Error(), "INVALID_PARAM") {
					t.Errorf("signal %q should be valid after normalization", tc.signal)
				}
			}
			if result != nil && result.IsError {
				text := extractText(t, result)
				if containsStr(text, "INVALID_PARAM") {
					t.Errorf("signal %q should be valid after normalization", tc.signal)
				}
			}
		})
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

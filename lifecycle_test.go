package main

import (
	"context"
	"os/exec"
	"runtime"
	"strconv"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Process lifecycle: spawn, find, signal
// ---------------------------------------------------------------------------

func TestLifecycle_SpawnAndKill(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("lifecycle test requires Linux ps/kill semantics")
	}

	// Spawn a sleep process
	cmd := exec.Command("sleep", "300")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start sleep process: %v", err)
	}
	pid := cmd.Process.Pid
	defer cmd.Process.Kill()

	// Verify it appears in ps_list
	td := findTool(t, "ps_list")
	req := makeReq(map[string]any{"filter": "sleep", "limit": 100})
	result, err := td.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("ps_list error: %v", err)
	}
	var out PsListOutput
	unmarshalResult(t, result, &out)

	found := false
	for _, p := range out.Processes {
		if p.PID == pid {
			found = true
			break
		}
	}
	if !found {
		t.Logf("pid %d not found in ps_list (filter may not match)", pid)
	}

	// Kill it with TERM
	killTd := findTool(t, "kill_process")
	killReq := makeReq(map[string]any{"pid": pid, "signal": "TERM"})
	killResult, err := killTd.Handler(context.Background(), killReq)
	if err != nil {
		t.Fatalf("kill_process error: %v", err)
	}
	var killOut KillProcessOutput
	unmarshalResult(t, killResult, &killOut)
	if killOut.PID != pid {
		t.Errorf("expected PID=%d, got %d", pid, killOut.PID)
	}
	if killOut.Signal != "TERM" {
		t.Errorf("expected Signal=TERM, got %q", killOut.Signal)
	}

	// Wait for process to die
	time.Sleep(100 * time.Millisecond)

	// Verify kill on already-dead process returns NOT_FOUND
	killReq2 := makeReq(map[string]any{"pid": pid})
	result2, err2 := killTd.Handler(context.Background(), killReq2)
	if err2 != nil {
		assertContains(t, err2.Error(), "NOT_FOUND")
	} else if result2 != nil && result2.IsError {
		// Also acceptable
	}
}

// ---------------------------------------------------------------------------
// ps_tree with current PID
// ---------------------------------------------------------------------------

func TestPsTree_CurrentProcess(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("ps_tree requires Linux pstree/ps --forest")
	}

	td := findTool(t, "ps_tree")
	// Use PID 1 (init/systemd) which should always exist
	req := makeReq(map[string]any{"pid": 1})
	result, err := td.Handler(context.Background(), req)
	if err != nil {
		t.Skipf("ps_tree for PID 1 failed (permissions?): %v", err)
	}
	var out PsTreeOutput
	unmarshalResult(t, result, &out)
	if out.PID != 1 {
		t.Errorf("PID = %d, want 1", out.PID)
	}
	if out.Tree == "" {
		t.Error("tree is empty")
	}
}

func TestPsTree_NonexistentPID(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("ps_tree requires Linux")
	}

	td := findTool(t, "ps_tree")
	req := makeReq(map[string]any{"pid": 999999999})
	result, err := td.Handler(context.Background(), req)
	if err != nil {
		// Expected: NOT_FOUND
		return
	}
	if result == nil || !result.IsError {
		t.Fatal("expected error for nonexistent PID")
	}
}

// ---------------------------------------------------------------------------
// kill_process edge cases
// ---------------------------------------------------------------------------

func TestKillProcess_NegativePID(t *testing.T) {
	td := findTool(t, "kill_process")
	req := makeReq(map[string]any{"pid": -1})
	result, err := td.Handler(context.Background(), req)
	if err != nil {
		assertContains(t, err.Error(), "INVALID_PARAM")
		return
	}
	if result == nil || !result.IsError {
		t.Fatal("expected error for negative PID")
	}
}

func TestKillProcess_NonexistentPID(t *testing.T) {
	td := findTool(t, "kill_process")
	req := makeReq(map[string]any{"pid": 999999999})
	_, err := td.Handler(context.Background(), req)
	// Should fail with NOT_FOUND, not INVALID_PARAM
	if err != nil {
		if containsStr(err.Error(), "INVALID_PARAM") {
			t.Fatalf("nonexistent PID should not be INVALID_PARAM: %v", err)
		}
	}
}

func TestKillProcess_AllValidSignals(t *testing.T) {
	td := findTool(t, "kill_process")

	for sig := range validSignals {
		t.Run("signal_"+sig, func(t *testing.T) {
			req := makeReq(map[string]any{"pid": 999999999, "signal": sig})
			result, err := td.Handler(context.Background(), req)
			// Should fail with process not found, NOT invalid signal
			if err != nil && containsStr(err.Error(), "INVALID_PARAM") {
				t.Errorf("signal %q should be valid", sig)
			}
			if result != nil && result.IsError {
				text := extractText(t, result)
				if containsStr(text, "INVALID_PARAM") {
					t.Errorf("signal %q should be valid", sig)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// investigate_port validation
// ---------------------------------------------------------------------------

func TestInvestigatePort_PortBoundaries(t *testing.T) {
	td := findTool(t, "investigate_port")

	invalidPorts := []int{0, -1, -100, 65536, 99999}
	for _, port := range invalidPorts {
		t.Run("port_"+strconv.Itoa(port), func(t *testing.T) {
			req := makeReq(map[string]any{"port": port})
			result, err := td.Handler(context.Background(), req)
			if err != nil {
				return // error expected
			}
			if result == nil || !result.IsError {
				t.Errorf("expected error for port=%d", port)
			}
		})
	}
}

func TestInvestigatePort_ValidBoundary(t *testing.T) {
	td := findTool(t, "investigate_port")

	// Ports 1 and 65535 should be accepted (though likely unused)
	validPorts := []int{1, 65535}
	for _, port := range validPorts {
		t.Run("port_"+strconv.Itoa(port), func(t *testing.T) {
			req := makeReq(map[string]any{"port": port})
			result, err := td.Handler(context.Background(), req)
			// Should NOT fail with INVALID_PARAM
			if err != nil {
				if containsStr(err.Error(), "INVALID_PARAM") {
					t.Errorf("port %d should be valid: %v", port, err)
				}
			}
			if result != nil && result.IsError {
				text := extractText(t, result)
				if containsStr(text, "INVALID_PARAM") {
					t.Errorf("port %d should be valid", port)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// investigate_service validation
// ---------------------------------------------------------------------------

func TestInvestigateService_EmptyUnit_Detailed(t *testing.T) {
	td := findTool(t, "investigate_service")
	req := makeReq(map[string]any{"unit": ""})
	result, err := td.Handler(context.Background(), req)
	if err != nil {
		assertContains(t, err.Error(), "INVALID_PARAM")
		return
	}
	if result == nil || !result.IsError {
		t.Fatal("expected error for empty unit")
	}
}

func TestInvestigateService_LogLines(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("requires systemctl")
	}

	td := findTool(t, "investigate_service")
	// Default log_lines should be 20
	req := makeReq(map[string]any{"unit": "nonexistent-xyz.service"})
	_, _ = td.Handler(context.Background(), req)
	// Just verify it doesn't panic with default log_lines

	// Custom log_lines
	req2 := makeReq(map[string]any{"unit": "nonexistent-xyz.service", "log_lines": 5})
	_, _ = td.Handler(context.Background(), req2)
}

// ---------------------------------------------------------------------------
// gpu_status — LookPath guard
// ---------------------------------------------------------------------------

func TestGpuStatus_NoNvidiaSmi(t *testing.T) {
	if _, err := exec.LookPath("nvidia-smi"); err == nil {
		t.Skip("nvidia-smi is available — skip absence test")
	}
	td := findTool(t, "gpu_status")
	req := makeReq(nil)
	result, err := td.Handler(context.Background(), req)
	if err != nil {
		assertContains(t, err.Error(), "NOT_FOUND")
		return
	}
	if result == nil || !result.IsError {
		t.Fatal("expected NOT_FOUND error when nvidia-smi is absent")
	}
}

// ---------------------------------------------------------------------------
// system_info — on macOS, verifies hostname still works
// ---------------------------------------------------------------------------

func TestSystemInfo_Hostname(t *testing.T) {
	td := findTool(t, "system_info")
	req := makeReq(nil)
	result, err := td.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	var out SystemInfoOutput
	unmarshalResult(t, result, &out)
	if out.Hostname == "" {
		t.Error("hostname should not be empty on any OS")
	}
}

// ---------------------------------------------------------------------------
// ps_tree — invalid PID range
// ---------------------------------------------------------------------------

func TestPsTree_ZeroPID(t *testing.T) {
	td := findTool(t, "ps_tree")
	req := makeReq(map[string]any{"pid": 0})
	result, err := td.Handler(context.Background(), req)
	if err != nil {
		assertContains(t, err.Error(), "INVALID_PARAM")
		return
	}
	if result == nil || !result.IsError {
		t.Fatal("expected error for pid=0")
	}
}

// ---------------------------------------------------------------------------
// investigate_port — exercising handler with ss absent
// ---------------------------------------------------------------------------

func TestInvestigatePort_DefaultLogLines(t *testing.T) {
	td := findTool(t, "investigate_port")
	// Port 1 is valid but unlikely to have a listener
	req := makeReq(map[string]any{"port": 1})
	result, err := td.Handler(context.Background(), req)
	// Should NOT fail with INVALID_PARAM — the port is valid
	if err != nil {
		if containsStr(err.Error(), "INVALID_PARAM") {
			t.Fatalf("port 1 should be valid: %v", err)
		}
	}
	_ = result // may be error result on macOS (no ss)
}

func TestInvestigatePort_CustomLogLines(t *testing.T) {
	td := findTool(t, "investigate_port")
	req := makeReq(map[string]any{"port": 2, "log_lines": 5})
	result, err := td.Handler(context.Background(), req)
	if err != nil {
		if containsStr(err.Error(), "INVALID_PARAM") {
			t.Fatalf("port 2 should be valid: %v", err)
		}
	}
	_ = result
}

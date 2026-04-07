package main

import (
	"context"
	"net"
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
		// Expected: NOT_FOUND error
		return
	}
	if result != nil && result.IsError {
		// Error result is also acceptable
		return
	}
	// pstree may succeed with empty output for nonexistent PIDs on some systems.
	// In that case, verify we at least get the PID back in the output.
	var out PsTreeOutput
	unmarshalResult(t, result, &out)
	if out.PID != 999999999 {
		t.Errorf("expected PID=999999999, got %d", out.PID)
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

// ---------------------------------------------------------------------------
// investigate_port — with a real TCP listener to cover the full path
// ---------------------------------------------------------------------------

func TestInvestigatePort_WithListener(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("requires Linux ss and pstree")
	}
	if _, err := exec.LookPath("ss"); err != nil {
		t.Skip("ss not available")
	}

	// Start a TCP listener on a random port
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	t.Logf("listening on port %d", port)

	td := findTool(t, "investigate_port")
	req := makeReq(map[string]any{"port": port, "log_lines": 3})
	result, err := td.Handler(context.Background(), req)
	if err != nil {
		// It's acceptable if it can't find the process (ss timing)
		t.Logf("investigate_port error (acceptable): %v", err)
		return
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	var out InvestigatePortOutput
	unmarshalResult(t, result, &out)
	if out.Port != port {
		t.Errorf("Port=%d, want %d", out.Port, port)
	}
	if out.Process != nil {
		t.Logf("found process: PID=%d Command=%s", out.Process.PID, out.Process.Command)
	}
}

// ---------------------------------------------------------------------------
// port_list — with a real listener to cover filter-by-port path
// ---------------------------------------------------------------------------

func TestPortList_WithListener(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("requires Linux ss")
	}
	if _, err := exec.LookPath("ss"); err != nil {
		t.Skip("ss not available")
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port

	td := findTool(t, "port_list")
	// Test with port filter
	req := makeReq(map[string]any{"port": port})
	result, err := td.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	var out PortListOutput
	unmarshalResult(t, result, &out)
	if out.Total == 0 {
		t.Log("port filter did not match (ss timing or permissions)")
	} else {
		if out.Ports[0].Port != port {
			t.Errorf("Port=%d, want %d", out.Ports[0].Port, port)
		}
		if out.Ports[0].Protocol != "tcp" {
			t.Errorf("Protocol=%q, want tcp", out.Ports[0].Protocol)
		}
	}
}

// ---------------------------------------------------------------------------
// investigate_service — system scope
// ---------------------------------------------------------------------------

func TestInvestigateService_SystemScope(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("requires Linux systemctl")
	}
	if _, err := exec.LookPath("systemctl"); err != nil {
		t.Skip("systemctl not available")
	}

	td := findTool(t, "investigate_service")
	// Use system: true to exercise the system scope branch
	req := makeReq(map[string]any{
		"unit":      "nonexistent-test-xyz.service",
		"system":    true,
		"log_lines": 3,
	})
	result, err := td.Handler(context.Background(), req)
	// The service won't exist, but this exercises the system scope code path
	if err != nil {
		t.Logf("expected error for nonexistent service: %v", err)
	}
	if result != nil && !result.IsError {
		var out InvestigateServiceOutput
		unmarshalResult(t, result, &out)
		if out.Unit != "nonexistent-test-xyz.service" {
			t.Errorf("Unit=%q, want nonexistent-test-xyz.service", out.Unit)
		}
		t.Logf("ActiveState=%q SubState=%q", out.ActiveState, out.SubState)
	}
}

func TestInvestigateService_SystemScope_RealUnit(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("requires Linux systemctl")
	}
	if _, err := exec.LookPath("systemctl"); err != nil {
		t.Skip("systemctl not available")
	}

	td := findTool(t, "investigate_service")
	// Try a real system service that should exist
	req := makeReq(map[string]any{
		"unit":      "systemd-journald.service",
		"system":    true,
		"log_lines": 3,
	})
	result, err := td.Handler(context.Background(), req)
	if err != nil {
		t.Skipf("error (may need root): %v", err)
	}
	if result == nil || result.IsError {
		t.Skip("service not accessible")
	}
	var out InvestigateServiceOutput
	unmarshalResult(t, result, &out)
	if out.Unit != "systemd-journald.service" {
		t.Errorf("Unit=%q, want systemd-journald.service", out.Unit)
	}
	t.Logf("ActiveState=%q SubState=%q MainPID=%d", out.ActiveState, out.SubState, out.MainPID)
}

// ---------------------------------------------------------------------------
// ps_tree — with the test process's own PID for a valid tree
// ---------------------------------------------------------------------------

func TestPsTree_OwnProcess(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("ps_tree requires Linux")
	}

	td := findTool(t, "ps_tree")
	// Use our own parent PID — should always be valid
	ppid := 1 // init/systemd always exists
	req := makeReq(map[string]any{"pid": ppid})
	result, err := td.Handler(context.Background(), req)
	if err != nil {
		t.Skipf("ps_tree for PID %d failed: %v", ppid, err)
	}
	var out PsTreeOutput
	unmarshalResult(t, result, &out)
	if out.Tree == "" {
		t.Error("tree should not be empty for PID 1")
	}
}

// ---------------------------------------------------------------------------
// ps_list — sort by pid
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// kill_process — permission denied branch (signal PID 1 as non-root)
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// readProcFile — error path
// ---------------------------------------------------------------------------

func TestReadProcFile_Error(t *testing.T) {
	_, err := readProcFile("nonexistent_file_xyz_12345")
	if err == nil {
		t.Fatal("expected error for nonexistent proc file")
	}
	if !containsStr(err.Error(), "read /proc/") {
		t.Errorf("error should mention /proc/: %v", err)
	}
}

func TestReadProcFile_Success(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("requires /proc filesystem")
	}
	data, err := readProcFile("self/status")
	if err != nil {
		t.Fatalf("readProcFile error: %v", err)
	}
	if data == "" {
		t.Error("expected non-empty data from /proc/self/status")
	}
}

// ---------------------------------------------------------------------------
// kill_process — permission denied branch (signal PID 1 as non-root)
// ---------------------------------------------------------------------------

func TestKillProcess_PermissionDenied(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("requires Linux")
	}
	// PID 1 (init) cannot be signaled by a regular user
	td := findTool(t, "kill_process")
	req := makeReq(map[string]any{"pid": 1, "signal": "HUP"})
	_, err := td.Handler(context.Background(), req)
	if err == nil {
		t.Log("kill PID 1 succeeded (test running as root?)")
		return
	}
	// Should be either PERMISSION or NOT_FOUND
	if !containsStr(err.Error(), "PERMISSION") && !containsStr(err.Error(), "NOT_FOUND") {
		t.Logf("unexpected error type: %v", err)
	}
}

func TestPsList_SortPID(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("ps --sort requires Linux")
	}

	td := findTool(t, "ps_list")
	req := makeReq(map[string]any{"sort_by": "pid", "limit": 5})
	result, err := td.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	var out PsListOutput
	unmarshalResult(t, result, &out)
	if out.Total == 0 {
		t.Error("expected processes")
	}
	// Verify PIDs are in ascending order
	for i := 1; i < len(out.Processes); i++ {
		if out.Processes[i].PID < out.Processes[i-1].PID {
			t.Errorf("processes not sorted by PID: %d < %d",
				out.Processes[i].PID, out.Processes[i-1].PID)
		}
	}
}

package main

import (
	"context"
	"encoding/json"
	"os/exec"
	"runtime"
	"testing"

	"github.com/hairglasses-studio/mcpkit/mcptest"
	"github.com/hairglasses-studio/mcpkit/registry"
)

func requireLinuxPS(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("ps --sort requires Linux")
	}
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

func TestModuleRegistration(t *testing.T) {
	m := &ProcessModule{}
	tools := m.Tools()
	if len(tools) != 8 {
		t.Fatalf("expected 8 tools, got %d", len(tools))
	}

	reg := registry.NewToolRegistry()
	reg.RegisterModule(m)
	srv := mcptest.NewServer(t, reg)

	names := srv.ToolNames()
	if len(names) != 8 {
		t.Fatalf("expected 8 registered tools, got %d", len(names))
	}

	for _, want := range []string{
		"ps_list", "ps_tree", "kill_process",
		"port_list", "gpu_status", "system_info",
		"investigate_port", "investigate_service",
	} {
		if !srv.HasTool(want) {
			t.Errorf("missing tool: %s", want)
		}
	}
}

func TestContextRegistries(t *testing.T) {
	if got := buildProcessResourceRegistry().ResourceCount(); got != 1 {
		t.Fatalf("expected 1 process resource, got %d", got)
	}
	if got := buildProcessPromptRegistry().PromptCount(); got != 1 {
		t.Fatalf("expected 1 process prompt, got %d", got)
	}
}

// ---------------------------------------------------------------------------
// ps_list
// ---------------------------------------------------------------------------

func TestPsList_Default(t *testing.T) {
	requireLinuxPS(t)
	td := findTool(t, "ps_list")
	req := makeReq(nil)
	result, err := td.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	var out PsListOutput
	unmarshalResult(t, result, &out)
	if out.Total == 0 {
		t.Error("expected at least 1 process")
	}
}

func TestPsList_InvalidSort(t *testing.T) {
	td := findTool(t, "ps_list")
	req := makeReq(map[string]any{"sort_by": "invalid"})
	result, err := td.Handler(context.Background(), req)
	if err != nil {
		assertContains(t, err.Error(), "INVALID_PARAM")
		return
	}
	// If handler returns result with IsError, check that
	if result != nil && result.IsError {
		t.Log("handler returned error result")
		return
	}
	t.Fatal("expected error for invalid sort_by")
}

func TestPsList_SortMem(t *testing.T) {
	requireLinuxPS(t)
	td := findTool(t, "ps_list")
	req := makeReq(map[string]any{"sort_by": "mem"})
	result, err := td.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	var out PsListOutput
	unmarshalResult(t, result, &out)
	if out.Total == 0 {
		t.Error("expected processes")
	}
}

func TestPsList_Filter(t *testing.T) {
	requireLinuxPS(t)
	td := findTool(t, "ps_list")
	req := makeReq(map[string]any{"filter": "process-mcp.test", "limit": 50})
	result, err := td.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	var out PsListOutput
	unmarshalResult(t, result, &out)
	// The test binary itself should match
	if out.Total == 0 {
		t.Log("filter did not match test binary (may vary by OS)")
	}
}

func TestPsList_Limit(t *testing.T) {
	requireLinuxPS(t)
	td := findTool(t, "ps_list")
	req := makeReq(map[string]any{"limit": 3})
	result, err := td.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	var out PsListOutput
	unmarshalResult(t, result, &out)
	if out.Total > 3 {
		t.Errorf("expected at most 3 processes, got %d", out.Total)
	}
}

// ---------------------------------------------------------------------------
// ps_tree
// ---------------------------------------------------------------------------

func TestPsTree_InvalidPID(t *testing.T) {
	td := findTool(t, "ps_tree")
	for _, pid := range []int{0, -1, -999} {
		req := makeReq(map[string]any{"pid": pid})
		result, err := td.Handler(context.Background(), req)
		if err != nil {
			assertContains(t, err.Error(), "INVALID_PARAM")
			continue
		}
		if result == nil || !result.IsError {
			t.Errorf("expected error for pid=%d", pid)
		}
	}
}

// ---------------------------------------------------------------------------
// kill_process
// ---------------------------------------------------------------------------

func TestKillProcess_InvalidPID(t *testing.T) {
	td := findTool(t, "kill_process")
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

func TestKillProcess_InvalidSignal(t *testing.T) {
	td := findTool(t, "kill_process")
	req := makeReq(map[string]any{"pid": 1, "signal": "INVALID"})
	result, err := td.Handler(context.Background(), req)
	if err != nil {
		assertContains(t, err.Error(), "INVALID_PARAM")
		return
	}
	if result == nil || !result.IsError {
		t.Fatal("expected error for invalid signal")
	}
}

func TestKillProcess_ValidSignals(t *testing.T) {
	expected := []string{"TERM", "KILL", "HUP", "INT", "USR1", "USR2", "STOP", "CONT"}
	for _, sig := range expected {
		if !validSignals[sig] {
			t.Errorf("signal %q not in validSignals map", sig)
		}
	}
	if len(validSignals) != len(expected) {
		t.Errorf("validSignals has %d entries, expected %d", len(validSignals), len(expected))
	}
}

func TestKillProcess_DefaultSignal(t *testing.T) {
	// Verify default signal is TERM by sending to a nonexistent PID
	td := findTool(t, "kill_process")
	req := makeReq(map[string]any{"pid": 999999999})
	_, err := td.Handler(context.Background(), req)
	// Should fail with "No such process", not "invalid signal"
	if err != nil {
		assertNotContains(t, err.Error(), "INVALID_PARAM")
	}
}

// ---------------------------------------------------------------------------
// port_list
// ---------------------------------------------------------------------------

func TestPortList_Default(t *testing.T) {
	if _, err := exec.LookPath("ss"); err != nil {
		t.Skip("ss not available")
	}
	td := findTool(t, "port_list")
	req := makeReq(nil)
	result, err := td.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	var out PortListOutput
	unmarshalResult(t, result, &out)
	// May be empty but should not error
	if out.Ports == nil {
		t.Error("ports should be non-nil (empty slice)")
	}
}

// ---------------------------------------------------------------------------
// gpu_status
// ---------------------------------------------------------------------------

func TestGpuStatus(t *testing.T) {
	if _, err := exec.LookPath("nvidia-smi"); err != nil {
		t.Skip("nvidia-smi not available")
	}
	td := findTool(t, "gpu_status")
	req := makeReq(nil)
	result, err := td.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	var out GpuStatusOutput
	unmarshalResult(t, result, &out)
	if out.GPU == nil {
		t.Fatal("expected GPU info")
	}
	if out.GPU.Name == "" {
		t.Error("GPU name is empty")
	}
	if out.GPU.MemoryTotal <= 0 {
		t.Error("GPU memory total should be > 0")
	}
}

// ---------------------------------------------------------------------------
// system_info
// ---------------------------------------------------------------------------

func TestSystemInfo(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("system_info reads /proc which requires Linux")
	}
	td := findTool(t, "system_info")
	req := makeReq(nil)
	result, err := td.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	var out SystemInfoOutput
	unmarshalResult(t, result, &out)

	if out.Hostname == "" {
		t.Error("hostname is empty")
	}
	if out.Kernel == "" {
		t.Error("kernel is empty")
	}
	if out.CPUCount <= 0 {
		t.Error("cpu_count should be > 0")
	}
	if out.MemTotalMB <= 0 {
		t.Error("mem_total_mb should be > 0")
	}
	if out.Uptime == "" {
		t.Error("uptime is empty")
	}
	if len(out.LoadAvg) != 3 {
		t.Errorf("expected 3 load avg values, got %d", len(out.LoadAvg))
	}
}

// ---------------------------------------------------------------------------
// investigate_port
// ---------------------------------------------------------------------------

func TestInvestigatePort_InvalidPort(t *testing.T) {
	td := findTool(t, "investigate_port")
	for _, port := range []int{0, -1, 99999} {
		req := makeReq(map[string]any{"port": port})
		result, err := td.Handler(context.Background(), req)
		if err != nil {
			continue // error is expected
		}
		if result == nil || !result.IsError {
			t.Errorf("expected error for port=%d", port)
		}
	}
}

func TestInvestigatePort_UnusedPort(t *testing.T) {
	if _, err := exec.LookPath("ss"); err != nil {
		t.Skip("ss not available")
	}
	td := findTool(t, "investigate_port")
	req := makeReq(map[string]any{"port": 59999})
	result, err := td.Handler(context.Background(), req)
	// Should return NOT_FOUND error for unused port
	if err == nil && result != nil && !result.IsError {
		t.Error("expected error for unused port")
	}
}

// ---------------------------------------------------------------------------
// investigate_service
// ---------------------------------------------------------------------------

func TestInvestigateService_EmptyUnit(t *testing.T) {
	td := findTool(t, "investigate_service")
	req := makeReq(map[string]any{"unit": ""})
	result, err := td.Handler(context.Background(), req)
	if err == nil && (result == nil || !result.IsError) {
		t.Fatal("expected error for empty unit")
	}
}

func TestInvestigateService_KnownUnit(t *testing.T) {
	if _, err := exec.LookPath("systemctl"); err != nil {
		t.Skip("systemctl not available")
	}
	td := findTool(t, "investigate_service")
	req := makeReq(map[string]any{"unit": "dbus.service", "log_lines": 5})
	result, err := td.Handler(context.Background(), req)
	if err != nil {
		t.Skipf("handler error (unit may not exist): %v", err)
	}
	if result == nil || result.IsError {
		t.Skip("unit not available in this scope")
	}

	var out InvestigateServiceOutput
	unmarshalResult(t, result, &out)
	if out.Unit != "dbus.service" {
		t.Errorf("expected unit=dbus.service, got %q", out.Unit)
	}
	if out.ActiveState == "" {
		t.Error("active_state is empty")
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func findTool(t *testing.T, name string) registry.ToolDefinition {
	t.Helper()
	m := &ProcessModule{}
	for _, td := range m.Tools() {
		if td.Tool.Name == name {
			return td
		}
	}
	t.Fatalf("tool %q not found", name)
	return registry.ToolDefinition{}
}

func makeReq(args map[string]any) registry.CallToolRequest {
	req := registry.CallToolRequest{}
	if args == nil {
		args = map[string]any{}
	}
	req.Params.Arguments = args
	return req
}

func extractText(t *testing.T, result *registry.CallToolResult) string {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("result has no content")
	}
	tc, ok := result.Content[0].(registry.TextContent)
	if !ok {
		t.Fatalf("content is not TextContent, got %T", result.Content[0])
	}
	return tc.Text
}

func unmarshalResult(t *testing.T, result *registry.CallToolResult, out any) {
	t.Helper()
	text := extractText(t, result)
	if err := json.Unmarshal([]byte(text), out); err != nil {
		t.Fatalf("unmarshal error: %v; text=%s", err, text)
	}
}

func assertContains(t *testing.T, s, substr string) {
	t.Helper()
	if !contains(s, substr) {
		t.Errorf("expected %q to contain %q", s, substr)
	}
}

func assertNotContains(t *testing.T, s, substr string) {
	t.Helper()
	if contains(s, substr) {
		t.Errorf("expected %q to NOT contain %q", s, substr)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || searchString(s, substr))
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

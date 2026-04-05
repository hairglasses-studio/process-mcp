package main

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// ---------------------------------------------------------------------------
// Resource module
// ---------------------------------------------------------------------------

func TestProcessResourceModule_Metadata(t *testing.T) {
	m := &processResourceModule{}
	if m.Name() != "process_context" {
		t.Errorf("Name() = %q, want %q", m.Name(), "process_context")
	}
	if m.Description() == "" {
		t.Error("Description() is empty")
	}
}

func TestProcessResourceModule_Resources(t *testing.T) {
	m := &processResourceModule{}
	resources := m.Resources()
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	rd := resources[0]
	if rd.Category != "workflow" {
		t.Errorf("Category = %q, want %q", rd.Category, "workflow")
	}
	if len(rd.Tags) == 0 {
		t.Error("expected tags")
	}

	// Call the handler
	contents, err := rd.Handler(context.Background(), mcp.ReadResourceRequest{})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if len(contents) != 1 {
		t.Fatalf("expected 1 content, got %d", len(contents))
	}
	tc, ok := contents[0].(mcp.TextResourceContents)
	if !ok {
		t.Fatalf("expected TextResourceContents, got %T", contents[0])
	}
	if tc.Text == "" {
		t.Error("resource text is empty")
	}
	if tc.URI != "process://workflows/service-investigation" {
		t.Errorf("URI = %q, want %q", tc.URI, "process://workflows/service-investigation")
	}
}

func TestProcessResourceModule_NilTemplates(t *testing.T) {
	m := &processResourceModule{}
	if m.Templates() != nil {
		t.Error("expected nil templates")
	}
}

// ---------------------------------------------------------------------------
// Prompt module
// ---------------------------------------------------------------------------

func TestProcessPromptModule_Metadata(t *testing.T) {
	m := &processPromptModule{}
	if m.Name() != "process_prompts" {
		t.Errorf("Name() = %q, want %q", m.Name(), "process_prompts")
	}
	if m.Description() == "" {
		t.Error("Description() is empty")
	}
}

func TestProcessPromptModule_Prompts(t *testing.T) {
	m := &processPromptModule{}
	prompts := m.Prompts()
	if len(prompts) != 1 {
		t.Fatalf("expected 1 prompt, got %d", len(prompts))
	}

	pd := prompts[0]
	if pd.Category != "workflow" {
		t.Errorf("Category = %q, want %q", pd.Category, "workflow")
	}
}

func TestProcessPrompt_Handler(t *testing.T) {
	m := &processPromptModule{}
	pd := m.Prompts()[0]

	req := mcp.GetPromptRequest{}
	req.Params.Arguments = map[string]string{
		"target": "nginx.service",
		"kind":   "service",
	}

	result, err := pd.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if result.Description == "" {
		t.Error("Description is empty")
	}
	if len(result.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result.Messages))
	}
}

func TestProcessPrompt_DefaultKind(t *testing.T) {
	m := &processPromptModule{}
	pd := m.Prompts()[0]

	// Test with no kind — should default to "service"
	req := mcp.GetPromptRequest{}
	req.Params.Arguments = map[string]string{
		"target": "8080",
	}

	result, err := pd.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if len(result.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result.Messages))
	}
}

func TestProcessPrompt_PortKind(t *testing.T) {
	m := &processPromptModule{}
	pd := m.Prompts()[0]

	req := mcp.GetPromptRequest{}
	req.Params.Arguments = map[string]string{
		"target": "3000",
		"kind":   "port",
	}

	result, err := pd.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
}

// ---------------------------------------------------------------------------
// Module metadata
// ---------------------------------------------------------------------------

func TestProcessModule_Metadata(t *testing.T) {
	m := &ProcessModule{}
	if m.Name() != "process" {
		t.Errorf("Name() = %q, want %q", m.Name(), "process")
	}
	if m.Description() == "" {
		t.Error("Description() is empty")
	}
}

func TestProcessModule_ToolCount(t *testing.T) {
	m := &ProcessModule{}
	tools := m.Tools()
	if len(tools) != 8 {
		t.Errorf("expected 8 tools, got %d", len(tools))
	}
}

func TestProcessModule_WriteFlags(t *testing.T) {
	m := &ProcessModule{}
	toolMap := make(map[string]bool)
	for _, td := range m.Tools() {
		toolMap[td.Tool.Name] = td.IsWrite
	}

	// Only kill_process should be a write tool
	if !toolMap["kill_process"] {
		t.Error("kill_process should be IsWrite=true")
	}

	readOnly := []string{"ps_list", "ps_tree", "port_list", "gpu_status",
		"system_info", "investigate_port", "investigate_service"}
	for _, name := range readOnly {
		if toolMap[name] {
			t.Errorf("%s should be IsWrite=false", name)
		}
	}
}

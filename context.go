package main

import (
	"context"
	"fmt"

	"github.com/hairglasses-studio/mcpkit/prompts"
	"github.com/hairglasses-studio/mcpkit/resources"
	"github.com/mark3labs/mcp-go/mcp"
)

type processResourceModule struct{}

func (m *processResourceModule) Name() string { return "process_context" }
func (m *processResourceModule) Description() string {
	return "Reusable process and service investigation context"
}

func (m *processResourceModule) Resources() []resources.ResourceDefinition {
	return []resources.ResourceDefinition{
		{
			Resource: mcp.NewResource(
				"process://workflows/service-investigation",
				"Service Investigation Workflow",
				mcp.WithResourceDescription("Compact guide for tracing a process, service, or port issue"),
				mcp.WithMIMEType("text/markdown"),
			),
			Handler: func(_ context.Context, _ mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
				return []mcp.ResourceContents{
					mcp.TextResourceContents{
						URI:      "process://workflows/service-investigation",
						MIMEType: "text/markdown",
						Text:     "1. Use `ps_list` or `port_list` to identify the target process.\n2. Use `ps_tree` when parent/child relationships matter.\n3. Use `investigate_port` or `investigate_service` for a bounded composed investigation with recent logs.\n4. Only use `kill_process` after confirming the exact PID and signal.",
					},
				}, nil
			},
			Category: "workflow",
			Tags:     []string{"process", "debugging", "workflow"},
		},
	}
}

func (m *processResourceModule) Templates() []resources.TemplateDefinition { return nil }

type processPromptModule struct{}

func (m *processPromptModule) Name() string { return "process_prompts" }
func (m *processPromptModule) Description() string {
	return "Prompt workflows for process and service debugging"
}

func (m *processPromptModule) Prompts() []prompts.PromptDefinition {
	return []prompts.PromptDefinition{
		{
			Prompt: mcp.NewPrompt(
				"process_debug_target",
				mcp.WithPromptDescription("Investigate a service or port with bounded outputs before any kill action"),
				mcp.WithArgument("target", mcp.RequiredArgument(), mcp.ArgumentDescription("Service unit or port to investigate")),
				mcp.WithArgument("kind", mcp.ArgumentDescription("service or port")),
			),
			Handler: func(_ context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
				target := req.Params.Arguments["target"]
				kind := req.Params.Arguments["kind"]
				if kind == "" {
					kind = "service"
				}
				return &mcp.GetPromptResult{
					Description: "Investigate process target " + target,
					Messages: []mcp.PromptMessage{
						mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(fmt.Sprintf(
							"Investigate the %s target %q. Prefer `investigate_service` or `investigate_port` for a bounded overview, then drill into `ps_tree`, `ps_list`, or `port_list` only if needed. Do not suggest `kill_process` unless the evidence clearly identifies the right PID and signal.",
							kind, target,
						))),
					},
				}, nil
			},
			Category: "workflow",
			Tags:     []string{"process", "workflow", "debugging"},
		},
	}
}

func buildProcessResourceRegistry() *resources.ResourceRegistry {
	reg := resources.NewResourceRegistry()
	reg.RegisterModule(&processResourceModule{})
	return reg
}

func buildProcessPromptRegistry() *prompts.PromptRegistry {
	reg := prompts.NewPromptRegistry()
	reg.RegisterModule(&processPromptModule{})
	return reg
}

# Roadmap

## Current State

process-mcp provides 8 tools for Linux process inspection and debugging via MCP. Includes two composed tools (`investigate_port`, `investigate_service`) that chain multiple operations into a single call. Reads directly from /proc where possible. Graceful degradation when optional tools (pstree, nvidia-smi) are missing. Built on mcpkit with stdio transport.

All tools functional and tested. MIT licensed, README and CLAUDE.md in place.

## Planned

### Phase 1 — Coverage & Safety
- Add integration tests using `mcptest.NewServer()`
- Allowlist/denylist for `kill_process` (prevent killing critical PIDs like init, sshd)
- Add `ps_search` tool — find processes by open file, network connection, or environment variable
- Improve `gpu_status` to support AMD GPUs via `rocm-smi`

### Phase 2 — Deeper Inspection
- `fd_list` — list open file descriptors for a PID (from /proc/PID/fd)
- `net_connections` — show all TCP/UDP connections for a process (from /proc/PID/net)
- `mem_map` — summarize memory mapping regions for a PID
- Add cgroup and namespace info to `system_info`

### Phase 3 — Composed Workflows
- `investigate_memory` — composed tool: top memory consumers + per-process breakdown + OOM risk
- `investigate_cpu` — composed tool: top CPU consumers + load trend + scheduling info

## Future Considerations
- eBPF-based tracing integration for deeper observability
- Container-aware process inspection (detect containerized processes, show container context)
- Historical snapshots — capture and compare process state over time

<!-- whiteclaw-rollout:start -->
## Whiteclaw-Derived Overhaul (2026-04-08)

This tranche applies the highest-value whiteclaw findings that fit this repo's real surface: engineer briefs, bounded skills/runbooks, searchable provenance, scoped MCP packaging, and explicit verification ladders.

### Strategic Focus
- Treat this repo as a public mirror with a real user-facing contract, not just a generic publish artifact.
- The whiteclaw backport should harden schema visibility, fallback behavior docs, and mirror parity for the process-investigation surface.
- Keep feature work anchored to the canonical `dotfiles/mcp/process-mcp` source-of-truth path.

### Recommended Work
- [ ] [Mirror contract] Keep the canonical-source mapping to `dotfiles/mcp/process-mcp` explicit and verifiable.
- [ ] [Schema snapshots] Snapshot the contracts for `investigate_port`, `investigate_service`, and related exported surfaces.
- [ ] [Fallback docs] Document `/proc` fallback behavior and any host/runtime prerequisites that affect investigation quality.
- [ ] [Publish verification] Add mirror smoke tests that prove the released artifact matches the canonical source surface.

### Rationale Snapshot
- Tier / lifecycle: `standalone` / `publish-mirror`
- Language profile: `Go`
- Visibility / sensitivity: `PUBLIC` / `public`
- Surface baseline: AGENTS=yes, skills=yes, codex=yes, mcp_manifest=configured, ralph=yes, roadmap=yes
- Whiteclaw transfers in scope: mirror contract, schema snapshots, /proc fallback docs, publish verification
- Live repo notes: AGENTS, skills, Codex config, configured .mcp.json, .ralph, 1 workflow(s)

<!-- whiteclaw-rollout:end -->

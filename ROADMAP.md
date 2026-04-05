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

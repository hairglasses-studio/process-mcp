# process-mcp

This repo uses [AGENTS.md](AGENTS.md) as the canonical instruction file. Treat this file as compatibility guidance for Claude-specific workflows.

MCP server for Linux process management: process listing, process trees, signal delivery, port inspection, NVIDIA GPU status, and system information. Built with [mcpkit](https://github.com/hairglasses-studio/mcpkit).

## Build & Test
```bash
go build ./...
go vet ./...
go test ./... -count=1
go install .
```

## Tools (8)

### Process Management (3)
- `ps_list` — List processes sorted by CPU/mem/pid, filter by command substring
- `ps_tree` — Show process tree for a PID (pstree, falls back to ps --forest)
- `kill_process` — Send signal to process (TERM, KILL, HUP, INT, USR1, USR2, STOP, CONT)

### Network (1)
- `port_list` — List listening TCP ports with process info via ss

### GPU (1)
- `gpu_status` — NVIDIA GPU status: temp, utilization, memory, power, running processes

### System (1)
- `system_info` — Hostname, kernel, uptime, load average, CPU count, memory, swap

### Composed Debugging (2)
- `investigate_port` — **Composed**: port → process → tree → systemd unit → logs. Single tool replaces 4+ sequential calls
- `investigate_service` — **Composed**: systemd status → process info → ports → logs. Single tool replaces 3-4 sequential calls

## Key Patterns
- Reads directly from /proc for system_info (no shell overhead)
- Graceful degradation: pstree falls back to ps --forest, nvidia-smi absence handled cleanly
- Signal validation: only allows safe, known signal names
- Structured error codes: INVALID_PARAM, NOT_FOUND, API_ERROR, PERMISSION_DENIED

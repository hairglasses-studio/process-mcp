# process-mcp

> **Mirror** -- Canonical development lives in [hairglasses-studio/dotfiles](https://github.com/hairglasses-studio/dotfiles) at `mcp/process-mcp/`. This repo is a publish mirror kept in parity for `go install` and MCP registry discovery.

[![Go Reference](https://pkg.go.dev/badge/github.com/hairglasses-studio/process-mcp.svg)](https://pkg.go.dev/github.com/hairglasses-studio/process-mcp)
[![Go Report Card](https://goreportcard.com/badge/github.com/hairglasses-studio/process-mcp)](https://goreportcard.com/report/github.com/hairglasses-studio/process-mcp)
[![CI](https://github.com/hairglasses-studio/process-mcp/actions/workflows/ci.yml/badge.svg)](https://github.com/hairglasses-studio/process-mcp/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Glama](https://glama.ai/mcp/servers/hairglasses-studio/process-mcp/badges/score.svg)](https://glama.ai/mcp/servers/hairglasses-studio/process-mcp)

MCP server for Linux process inspection and debugging. Gives AI assistants like Codex or Claude Code the ability to list processes, investigate ports, check GPU status, and debug services — including composed tools that chain multiple operations into a single call.

Built with [mcpkit](https://github.com/hairglasses-studio/mcpkit) using stdio transport.

## Install

```bash
go install github.com/hairglasses-studio/process-mcp@latest
```

Or build from source:

```bash
git clone https://github.com/hairglasses-studio/process-mcp
cd process-mcp
go build -o process-mcp .
```

## Configure

Add to your MCP client config (for example Codex or Claude Code):

```json
{
  "mcpServers": {
    "process": {
      "command": "process-mcp"
    }
  }
}
```

## Tools

### Process Management
| Tool | Description |
|------|-------------|
| `ps_list` | List processes sorted by CPU/mem/pid, filter by command substring |
| `ps_tree` | Show process tree for a PID (pstree, falls back to ps --forest) |
| `kill_process` | Send signal to process (TERM, KILL, HUP, INT, USR1, USR2, STOP, CONT) |

### Network
| Tool | Description |
|------|-------------|
| `port_list` | List listening TCP ports with process info via `ss` |

### GPU
| Tool | Description |
|------|-------------|
| `gpu_status` | NVIDIA GPU status: temp, utilization, memory, power, running processes |

### System
| Tool | Description |
|------|-------------|
| `system_info` | Hostname, kernel, uptime, load average, CPU count, memory, swap |

### Composed Debugging
| Tool | Description |
|------|-------------|
| `investigate_port` | Port -> process -> tree -> systemd unit -> logs. Replaces 4+ sequential calls. |
| `investigate_service` | Systemd status -> process info -> ports -> logs. Replaces 3-4 sequential calls. |

## Composed Tools

The composed debugging tools are the key differentiator. Instead of manually chaining tool calls to investigate an issue, a single call gathers all relevant context:

```
"Something is using port 8080, can you figure out what?"
→ investigate_port(port: 8080)
  Returns: process name, PID, tree, systemd unit, recent logs — all in one response

"The nginx service seems unhealthy, debug it"
→ investigate_service(service: "nginx")
  Returns: systemd status, process info, listening ports, recent logs
```

## Key Patterns

- **Direct /proc reads**: `system_info` reads from `/proc` directly for efficiency (no shell overhead)
- **Graceful degradation**: `pstree` falls back to `ps --forest`; missing `nvidia-smi` handled cleanly
- **Signal validation**: Only allows known, safe signal names (no arbitrary signal numbers)
- **Structured error codes**: `INVALID_PARAM`, `NOT_FOUND`, `API_ERROR`, `PERMISSION_DENIED`

## License

MIT

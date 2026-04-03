// Command process-mcp is an MCP server for Linux process management,
// port inspection, GPU status, and system information via the Model
// Context Protocol (stdio transport).
//
// Usage:
//
//	process-mcp
package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/hairglasses-studio/mcpkit/handler"
	"github.com/hairglasses-studio/mcpkit/registry"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func runCmd(name string, args ...string) (string, string, error) {
	cmd := exec.Command(name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()), err
}

func readProcFile(name string) (string, error) {
	data, err := os.ReadFile("/proc/" + name)
	if err != nil {
		return "", fmt.Errorf("read /proc/%s: %w", name, err)
	}
	return strings.TrimSpace(string(data)), nil
}

// ---------------------------------------------------------------------------
// ps_list
// ---------------------------------------------------------------------------

type PsListInput struct {
	Filter string `json:"filter,omitempty" jsonschema:"description=Filter processes by command substring"`
	SortBy string `json:"sort_by,omitempty" jsonschema:"description=Sort by: cpu (default)\\, mem\\, or pid"`
	Limit  int    `json:"limit,omitempty" jsonschema:"description=Max processes to return. Default 20."`
}

type ProcessInfo struct {
	User    string  `json:"user"`
	PID     int     `json:"pid"`
	CPU     float64 `json:"cpu"`
	Mem     float64 `json:"mem"`
	VSZ     int     `json:"vsz"`
	RSS     int     `json:"rss"`
	TTY     string  `json:"tty"`
	Stat    string  `json:"stat"`
	Start   string  `json:"start"`
	Time    string  `json:"time"`
	Command string  `json:"command"`
}

type PsListOutput struct {
	Processes []ProcessInfo `json:"processes"`
	Total     int           `json:"total"`
}

// ---------------------------------------------------------------------------
// ps_tree
// ---------------------------------------------------------------------------

type PsTreeInput struct {
	PID int `json:"pid" jsonschema:"description=Process ID to show tree for,required"`
}

type PsTreeOutput struct {
	PID  int    `json:"pid"`
	Tree string `json:"tree"`
}

// ---------------------------------------------------------------------------
// kill_process
// ---------------------------------------------------------------------------

type KillProcessInput struct {
	PID    int    `json:"pid" jsonschema:"description=Process ID to signal,required"`
	Signal string `json:"signal,omitempty" jsonschema:"description=Signal name: TERM (default)\\, KILL\\, HUP\\, INT\\, USR1\\, USR2\\, STOP\\, CONT"`
}

type KillProcessOutput struct {
	PID    int    `json:"pid"`
	Signal string `json:"signal"`
	Result string `json:"result"`
}

var validSignals = map[string]bool{
	"TERM": true, "KILL": true, "HUP": true, "INT": true,
	"USR1": true, "USR2": true, "STOP": true, "CONT": true,
}

// ---------------------------------------------------------------------------
// port_list
// ---------------------------------------------------------------------------

type PortListInput struct {
	Port int `json:"port,omitempty" jsonschema:"description=Filter by specific port number"`
}

type PortEntry struct {
	Protocol string `json:"protocol"`
	Address  string `json:"address"`
	Port     int    `json:"port"`
	Process  string `json:"process"`
}

type PortListOutput struct {
	Ports []PortEntry `json:"ports"`
	Total int         `json:"total"`
}

// ---------------------------------------------------------------------------
// gpu_status
// ---------------------------------------------------------------------------

type GpuStatusInput struct{}

type GpuInfo struct {
	DriverVersion string  `json:"driver_version"`
	Name          string  `json:"name"`
	Temperature   int     `json:"temperature"`
	Utilization   int     `json:"utilization"`
	MemoryUsed    int     `json:"memory_used_mb"`
	MemoryTotal   int     `json:"memory_total_mb"`
	PowerDraw     float64 `json:"power_draw_w"`
}

type GpuProcess struct {
	PID        int    `json:"pid"`
	Name       string `json:"name"`
	MemoryUsed int    `json:"memory_used_mb"`
}

type GpuStatusOutput struct {
	GPU       *GpuInfo     `json:"gpu,omitempty"`
	Processes []GpuProcess `json:"processes"`
}

// ---------------------------------------------------------------------------
// system_info
// ---------------------------------------------------------------------------

type SystemInfoInput struct{}

type SystemInfoOutput struct {
	Hostname     string    `json:"hostname"`
	Kernel       string    `json:"kernel"`
	Uptime       string    `json:"uptime"`
	LoadAvg      []float64 `json:"load_avg"`
	CPUCount     int       `json:"cpu_count"`
	MemTotalMB   int       `json:"mem_total_mb"`
	MemAvailMB   int       `json:"mem_available_mb"`
	SwapTotalMB  int       `json:"swap_total_mb"`
	SwapUsedMB   int       `json:"swap_used_mb"`
}

// ---------------------------------------------------------------------------
// ProcessModule
// ---------------------------------------------------------------------------

type ProcessModule struct{}

func (m *ProcessModule) Name() string        { return "process" }
func (m *ProcessModule) Description() string { return "Linux process management, ports, GPU, and system info" }

func (m *ProcessModule) Tools() []registry.ToolDefinition {
	return []registry.ToolDefinition{
		// ---------------------------------------------------------------
		// ps_list
		// ---------------------------------------------------------------
		handler.TypedHandler[PsListInput, PsListOutput](
			"ps_list",
			"List running processes sorted by CPU, memory, or PID. Optionally filter by command substring.",
			func(_ context.Context, input PsListInput) (PsListOutput, error) {
				sortFlag := "--sort=-pcpu"
				switch input.SortBy {
				case "mem":
					sortFlag = "--sort=-pmem"
				case "pid":
					sortFlag = "--sort=pid"
				case "cpu", "":
					// default
				default:
					return PsListOutput{}, fmt.Errorf("[%s] sort_by must be cpu, mem, or pid", handler.ErrInvalidParam)
				}

				limit := input.Limit
				if limit <= 0 {
					limit = 20
				}

				out, _, err := runCmd("ps", "aux", sortFlag)
				if err != nil {
					return PsListOutput{}, fmt.Errorf("[%s] ps command failed: %w", handler.ErrAPIError, err)
				}

				lines := strings.Split(out, "\n")
				var processes []ProcessInfo
				for i, line := range lines {
					if i == 0 {
						continue // skip header
					}
					fields := strings.Fields(line)
					if len(fields) < 11 {
						continue
					}

					command := strings.Join(fields[10:], " ")

					if input.Filter != "" && !strings.Contains(strings.ToLower(command), strings.ToLower(input.Filter)) {
						continue
					}

					pid, _ := strconv.Atoi(fields[1])
					cpu, _ := strconv.ParseFloat(fields[2], 64)
					mem, _ := strconv.ParseFloat(fields[3], 64)
					vsz, _ := strconv.Atoi(fields[4])
					rss, _ := strconv.Atoi(fields[5])

					processes = append(processes, ProcessInfo{
						User:    fields[0],
						PID:     pid,
						CPU:     cpu,
						Mem:     mem,
						VSZ:     vsz,
						RSS:     rss,
						TTY:     fields[6],
						Stat:    fields[7],
						Start:   fields[8],
						Time:    fields[9],
						Command: command,
					})

					if len(processes) >= limit {
						break
					}
				}

				if processes == nil {
					processes = []ProcessInfo{}
				}

				return PsListOutput{Processes: processes, Total: len(processes)}, nil
			},
		),

		// ---------------------------------------------------------------
		// ps_tree
		// ---------------------------------------------------------------
		handler.TypedHandler[PsTreeInput, PsTreeOutput](
			"ps_tree",
			"Show process tree for a given PID using pstree. Falls back to ps --forest if pstree is unavailable.",
			func(_ context.Context, input PsTreeInput) (PsTreeOutput, error) {
				if input.PID <= 0 {
					return PsTreeOutput{}, fmt.Errorf("[%s] pid must be a positive integer", handler.ErrInvalidParam)
				}

				pidStr := strconv.Itoa(input.PID)

				// Try pstree first
				out, _, err := runCmd("pstree", "-p", pidStr)
				if err == nil {
					return PsTreeOutput{PID: input.PID, Tree: out}, nil
				}

				// Fallback to ps --forest
				out, _, err = runCmd("ps", "-ef", "--forest")
				if err != nil {
					return PsTreeOutput{}, fmt.Errorf("[%s] ps --forest failed: %w", handler.ErrAPIError, err)
				}

				var filtered []string
				for _, line := range strings.Split(out, "\n") {
					if strings.Contains(line, pidStr) {
						filtered = append(filtered, line)
					}
				}

				if len(filtered) == 0 {
					return PsTreeOutput{}, fmt.Errorf("[%s] process not found: pid %d", handler.ErrNotFound, input.PID)
				}

				return PsTreeOutput{PID: input.PID, Tree: strings.Join(filtered, "\n")}, nil
			},
		),

		// ---------------------------------------------------------------
		// kill_process
		// ---------------------------------------------------------------
		handler.TypedHandler[KillProcessInput, KillProcessOutput](
			"kill_process",
			"Send a signal to a process. Supports TERM, KILL, HUP, INT, USR1, USR2, STOP, CONT.",
			func(_ context.Context, input KillProcessInput) (KillProcessOutput, error) {
				if input.PID <= 0 {
					return KillProcessOutput{}, fmt.Errorf("[%s] pid must be a positive integer", handler.ErrInvalidParam)
				}

				sig := input.Signal
				if sig == "" {
					sig = "TERM"
				}
				sig = strings.ToUpper(sig)

				if !validSignals[sig] {
					return KillProcessOutput{}, fmt.Errorf("[%s] invalid signal %q; must be one of: TERM, KILL, HUP, INT, USR1, USR2, STOP, CONT", handler.ErrInvalidParam, sig)
				}

				pidStr := strconv.Itoa(input.PID)
				_, stderr, err := runCmd("kill", "-"+sig, pidStr)
				if err != nil {
					if strings.Contains(stderr, "No such process") {
						return KillProcessOutput{}, fmt.Errorf("[%s] process not found: pid %d", handler.ErrNotFound, input.PID)
					}
					if strings.Contains(stderr, "Operation not permitted") {
						return KillProcessOutput{}, fmt.Errorf("[%s] permission denied: pid %d", handler.ErrPermission, input.PID)
					}
					return KillProcessOutput{}, fmt.Errorf("[%s] kill failed: %s", handler.ErrAPIError, stderr)
				}

				return KillProcessOutput{
					PID:    input.PID,
					Signal: sig,
					Result: fmt.Sprintf("sent %s to pid %d", sig, input.PID),
				}, nil
			},
		),

		// ---------------------------------------------------------------
		// port_list
		// ---------------------------------------------------------------
		handler.TypedHandler[PortListInput, PortListOutput](
			"port_list",
			"List listening TCP ports with process info via ss. Optionally filter by port number.",
			func(_ context.Context, input PortListInput) (PortListOutput, error) {
				out, _, err := runCmd("ss", "-tlnp")
				if err != nil {
					return PortListOutput{}, fmt.Errorf("[%s] ss command failed: %w", handler.ErrAPIError, err)
				}

				lines := strings.Split(out, "\n")
				var ports []PortEntry

				for i, line := range lines {
					if i < 1 {
						continue // skip header
					}
					fields := strings.Fields(line)
					if len(fields) < 5 {
						continue
					}
					if fields[0] != "LISTEN" {
						continue
					}

					// Parse local address:port from field 3
					localAddr := fields[3]
					lastColon := strings.LastIndex(localAddr, ":")
					if lastColon < 0 {
						continue
					}

					addr := localAddr[:lastColon]
					portNum, err := strconv.Atoi(localAddr[lastColon+1:])
					if err != nil {
						continue
					}

					if input.Port > 0 && portNum != input.Port {
						continue
					}

					// Parse process info from the last field if it contains users:
					process := ""
					for _, f := range fields {
						if strings.HasPrefix(f, "users:") {
							process = f
							break
						}
					}

					ports = append(ports, PortEntry{
						Protocol: "tcp",
						Address:  addr,
						Port:     portNum,
						Process:  process,
					})
				}

				if ports == nil {
					ports = []PortEntry{}
				}

				return PortListOutput{Ports: ports, Total: len(ports)}, nil
			},
		),

		// ---------------------------------------------------------------
		// gpu_status
		// ---------------------------------------------------------------
		handler.TypedHandler[GpuStatusInput, GpuStatusOutput](
			"gpu_status",
			"Query NVIDIA GPU status: driver, temperature, utilization, memory, power draw, and running GPU processes.",
			func(_ context.Context, _ GpuStatusInput) (GpuStatusOutput, error) {
				// Check if nvidia-smi exists
				_, err := exec.LookPath("nvidia-smi")
				if err != nil {
					return GpuStatusOutput{}, fmt.Errorf("[%s] nvidia-smi not found: install NVIDIA drivers for GPU monitoring", handler.ErrNotFound)
				}

				var result GpuStatusOutput

				// Query GPU info
				out, stderr, err := runCmd("nvidia-smi",
					"--query-gpu=driver_version,name,temperature.gpu,utilization.gpu,memory.used,memory.total,power.draw",
					"--format=csv,noheader,nounits")
				if err != nil {
					return GpuStatusOutput{}, fmt.Errorf("[%s] nvidia-smi query failed: %s", handler.ErrAPIError, stderr)
				}

				fields := strings.Split(out, ", ")
				if len(fields) >= 7 {
					temp, _ := strconv.Atoi(strings.TrimSpace(fields[2]))
					util, _ := strconv.Atoi(strings.TrimSpace(fields[3]))
					memUsed, _ := strconv.Atoi(strings.TrimSpace(fields[4]))
					memTotal, _ := strconv.Atoi(strings.TrimSpace(fields[5]))
					power, _ := strconv.ParseFloat(strings.TrimSpace(fields[6]), 64)

					result.GPU = &GpuInfo{
						DriverVersion: strings.TrimSpace(fields[0]),
						Name:          strings.TrimSpace(fields[1]),
						Temperature:   temp,
						Utilization:   util,
						MemoryUsed:    memUsed,
						MemoryTotal:   memTotal,
						PowerDraw:     power,
					}
				}

				// Query GPU processes
				out, _, err = runCmd("nvidia-smi",
					"--query-compute-apps=pid,name,used_memory",
					"--format=csv,noheader,nounits")
				if err == nil && out != "" {
					for _, line := range strings.Split(out, "\n") {
						pfields := strings.Split(line, ", ")
						if len(pfields) >= 3 {
							pid, _ := strconv.Atoi(strings.TrimSpace(pfields[0]))
							mem, _ := strconv.Atoi(strings.TrimSpace(pfields[2]))
							result.Processes = append(result.Processes, GpuProcess{
								PID:        pid,
								Name:       strings.TrimSpace(pfields[1]),
								MemoryUsed: mem,
							})
						}
					}
				}

				if result.Processes == nil {
					result.Processes = []GpuProcess{}
				}

				return result, nil
			},
		),

		// ---------------------------------------------------------------
		// system_info
		// ---------------------------------------------------------------
		handler.TypedHandler[SystemInfoInput, SystemInfoOutput](
			"system_info",
			"Show system information: hostname, kernel, uptime, load average, CPU count, memory, and swap.",
			func(_ context.Context, _ SystemInfoInput) (SystemInfoOutput, error) {
				var info SystemInfoOutput

				// Hostname
				info.Hostname, _ = os.Hostname()

				// Kernel version from /proc/version
				if ver, err := readProcFile("version"); err == nil {
					fields := strings.Fields(ver)
					if len(fields) >= 3 {
						info.Kernel = fields[2] // "Linux version X.Y.Z-..."
					}
				}

				// Uptime from /proc/uptime
				if raw, err := readProcFile("uptime"); err == nil {
					fields := strings.Fields(raw)
					if len(fields) >= 1 {
						secs, _ := strconv.ParseFloat(fields[0], 64)
						totalSecs := int(secs)
						days := totalSecs / 86400
						hours := (totalSecs % 86400) / 3600
						mins := (totalSecs % 3600) / 60
						info.Uptime = fmt.Sprintf("%dd %dh %dm", days, hours, mins)
					}
				}

				// Load average from /proc/loadavg
				if raw, err := readProcFile("loadavg"); err == nil {
					fields := strings.Fields(raw)
					if len(fields) >= 3 {
						info.LoadAvg = make([]float64, 3)
						info.LoadAvg[0], _ = strconv.ParseFloat(fields[0], 64)
						info.LoadAvg[1], _ = strconv.ParseFloat(fields[1], 64)
						info.LoadAvg[2], _ = strconv.ParseFloat(fields[2], 64)
					}
				}

				// CPU count from /proc/cpuinfo
				if raw, err := readProcFile("cpuinfo"); err == nil {
					for _, line := range strings.Split(raw, "\n") {
						if strings.HasPrefix(line, "processor") {
							info.CPUCount++
						}
					}
				}

				// Memory from /proc/meminfo
				if raw, err := readProcFile("meminfo"); err == nil {
					memMap := make(map[string]int)
					for _, line := range strings.Split(raw, "\n") {
						if strings.HasPrefix(line, "MemTotal:") ||
							strings.HasPrefix(line, "MemAvailable:") ||
							strings.HasPrefix(line, "SwapTotal:") ||
							strings.HasPrefix(line, "SwapFree:") {
							parts := strings.Fields(line)
							if len(parts) >= 2 {
								val, _ := strconv.Atoi(parts[1])
								key := strings.TrimSuffix(parts[0], ":")
								memMap[key] = val
							}
						}
					}
					info.MemTotalMB = memMap["MemTotal"] / 1024
					info.MemAvailMB = memMap["MemAvailable"] / 1024
					info.SwapTotalMB = memMap["SwapTotal"] / 1024
					info.SwapUsedMB = (memMap["SwapTotal"] - memMap["SwapFree"]) / 1024
				}

				return info, nil
			},
		),
	}
}

// ---------------------------------------------------------------------------
// main
// ---------------------------------------------------------------------------

func main() {
	reg := registry.NewToolRegistry()
	reg.RegisterModule(&ProcessModule{})

	s := registry.NewMCPServer("process-mcp", "1.0.0")
	reg.RegisterWithServer(s)

	if err := registry.ServeStdio(s); err != nil {
		log.Fatal(err)
	}
}

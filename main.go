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
	"log/slog"
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
	Hostname    string    `json:"hostname"`
	Kernel      string    `json:"kernel"`
	Uptime      string    `json:"uptime"`
	LoadAvg     []float64 `json:"load_avg"`
	CPUCount    int       `json:"cpu_count"`
	MemTotalMB  int       `json:"mem_total_mb"`
	MemAvailMB  int       `json:"mem_available_mb"`
	SwapTotalMB int       `json:"swap_total_mb"`
	SwapUsedMB  int       `json:"swap_used_mb"`
}

// ---------------------------------------------------------------------------
// investigate_port
// ---------------------------------------------------------------------------

type InvestigatePortInput struct {
	Port     int `json:"port" jsonschema:"required,description=TCP port number to investigate"`
	LogLines int `json:"log_lines,omitempty" jsonschema:"description=Number of journal log lines to fetch. Default 20."`
}

type InvestigatePortOutput struct {
	Port          int          `json:"port"`
	Process       *ProcessInfo `json:"process,omitempty"`
	Tree          string       `json:"tree,omitempty"`
	SystemdUnit   string       `json:"systemd_unit,omitempty"`
	SystemdStatus string       `json:"systemd_status,omitempty"`
	RecentLogs    string       `json:"recent_logs,omitempty"`
}

// ---------------------------------------------------------------------------
// investigate_service
// ---------------------------------------------------------------------------

type InvestigateServiceInput struct {
	Unit     string `json:"unit" jsonschema:"required,description=Systemd unit name to investigate"`
	System   bool   `json:"system,omitempty" jsonschema:"description=Target system scope instead of user scope. Default: user scope."`
	LogLines int    `json:"log_lines,omitempty" jsonschema:"description=Number of journal log lines to fetch. Default 20."`
}

type InvestigateServiceOutput struct {
	Unit        string       `json:"unit"`
	ActiveState string       `json:"active_state"`
	SubState    string       `json:"sub_state"`
	MainPID     int          `json:"main_pid,omitempty"`
	Process     *ProcessInfo `json:"process,omitempty"`
	Ports       []PortEntry  `json:"ports,omitempty"`
	RecentLogs  string       `json:"recent_logs,omitempty"`
}

// ---------------------------------------------------------------------------
// ProcessModule
// ---------------------------------------------------------------------------

type ProcessModule struct{}

func (m *ProcessModule) Name() string { return "process" }
func (m *ProcessModule) Description() string {
	return "Linux process management, ports, GPU, and system info"
}

func (m *ProcessModule) Tools() []registry.ToolDefinition {
	psList := handler.TypedHandler[PsListInput, PsListOutput](
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
					continue
				}
				p, ok := parsePsLine(line)
				if !ok {
					continue
				}
				if input.Filter != "" && !strings.Contains(strings.ToLower(p.Command), strings.ToLower(input.Filter)) {
					continue
				}
				processes = append(processes, p)
				if len(processes) >= limit {
					break
				}
			}

			if processes == nil {
				processes = []ProcessInfo{}
			}
			return PsListOutput{Processes: processes, Total: len(processes)}, nil
		},
	)
	psList.SearchTerms = []string{"top processes", "running processes", "cpu usage", "memory usage"}

	psTree := handler.TypedHandler[PsTreeInput, PsTreeOutput](
		"ps_tree",
		"Show process tree for a given PID using pstree. Falls back to ps --forest if pstree is unavailable.",
		func(_ context.Context, input PsTreeInput) (PsTreeOutput, error) {
			if input.PID <= 0 {
				return PsTreeOutput{}, fmt.Errorf("[%s] pid must be a positive integer", handler.ErrInvalidParam)
			}

			pidStr := strconv.Itoa(input.PID)
			out, _, err := runCmd("pstree", "-p", pidStr)
			if err == nil {
				return PsTreeOutput{PID: input.PID, Tree: out}, nil
			}

			out, _, err = runCmd("ps", "-ef", "--forest")
			if err != nil {
				return PsTreeOutput{}, fmt.Errorf("[%s] ps --forest failed: %w", handler.ErrAPIError, err)
			}

			var filtered []string
			for line := range strings.SplitSeq(out, "\n") {
				if strings.Contains(line, pidStr) {
					filtered = append(filtered, line)
				}
			}
			if len(filtered) == 0 {
				return PsTreeOutput{}, fmt.Errorf("[%s] process not found: pid %d", handler.ErrNotFound, input.PID)
			}

			return PsTreeOutput{PID: input.PID, Tree: strings.Join(filtered, "\n")}, nil
		},
	)
	psTree.MaxResultChars = 8000

	killProcess := handler.TypedHandler[KillProcessInput, KillProcessOutput](
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

			slog.Info("sending signal", "pid", input.PID, "signal", sig)
			pidStr := strconv.Itoa(input.PID)
			_, stderr, err := runCmd("kill", "-"+sig, pidStr)
			if err != nil {
				if strings.Contains(stderr, "No such process") {
					slog.Error("process not found", "pid", input.PID, "signal", sig)
					return KillProcessOutput{}, fmt.Errorf("[%s] process not found: pid %d", handler.ErrNotFound, input.PID)
				}
				if strings.Contains(stderr, "Operation not permitted") {
					slog.Error("permission denied", "pid", input.PID, "signal", sig)
					return KillProcessOutput{}, fmt.Errorf("[%s] permission denied: pid %d", handler.ErrPermission, input.PID)
				}
				slog.Error("kill failed", "pid", input.PID, "signal", sig, "error", stderr)
				return KillProcessOutput{}, fmt.Errorf("[%s] kill failed: %s", handler.ErrAPIError, stderr)
			}

			slog.Info("signal sent", "pid", input.PID, "signal", sig)
			return KillProcessOutput{
				PID:    input.PID,
				Signal: sig,
				Result: fmt.Sprintf("sent %s to pid %d", sig, input.PID),
			}, nil
		},
	)
	killProcess.IsWrite = true

	portList := handler.TypedHandler[PortListInput, PortListOutput](
		"port_list",
		"List listening TCP ports with process info via ss. Optionally filter by port number.",
		func(_ context.Context, input PortListInput) (PortListOutput, error) {
			out, _, err := runCmd("ss", "-tlnp")
			if err != nil {
				return PortListOutput{}, fmt.Errorf("[%s] ss command failed: %w", handler.ErrAPIError, err)
			}

			ports := parseSsLines(out, input.Port)
			if ports == nil {
				ports = []PortEntry{}
			}
			return PortListOutput{Ports: ports, Total: len(ports)}, nil
		},
	)
	portList.SearchTerms = []string{"open ports", "listening ports", "network sockets"}

	gpuStatus := handler.TypedHandler[GpuStatusInput, GpuStatusOutput](
		"gpu_status",
		"Query NVIDIA GPU status: driver, temperature, utilization, memory, power draw, and running GPU processes.",
		func(_ context.Context, _ GpuStatusInput) (GpuStatusOutput, error) {
			_, err := exec.LookPath("nvidia-smi")
			if err != nil {
				return GpuStatusOutput{}, fmt.Errorf("[%s] nvidia-smi not found: install NVIDIA drivers for GPU monitoring", handler.ErrNotFound)
			}

			var result GpuStatusOutput
			out, stderr, err := runCmd("nvidia-smi",
				"--query-gpu=driver_version,name,temperature.gpu,utilization.gpu,memory.used,memory.total,power.draw",
				"--format=csv,noheader,nounits")
			if err != nil {
				return GpuStatusOutput{}, fmt.Errorf("[%s] nvidia-smi query failed: %s", handler.ErrAPIError, stderr)
			}

			result.GPU = parseGpuInfo(out)

			out, _, err = runCmd("nvidia-smi", "--query-compute-apps=pid,name,used_memory", "--format=csv,noheader,nounits")
			if err == nil && out != "" {
				for line := range strings.SplitSeq(out, "\n") {
					if gp, ok := parseGpuProcessLine(line); ok {
						result.Processes = append(result.Processes, gp)
					}
				}
			}

			if result.Processes == nil {
				result.Processes = []GpuProcess{}
			}
			return result, nil
		},
	)
	gpuStatus.SearchTerms = []string{"nvidia", "gpu usage", "gpu processes", "cuda processes"}

	systemInfo := handler.TypedHandler[SystemInfoInput, SystemInfoOutput](
		"system_info",
		"Show system information: hostname, kernel, uptime, load average, CPU count, memory, and swap.",
		func(_ context.Context, _ SystemInfoInput) (SystemInfoOutput, error) {
			var info SystemInfoOutput
			info.Hostname, _ = os.Hostname()

			if ver, err := readProcFile("version"); err == nil {
				info.Kernel = parseKernelVersion(ver)
			}
			if raw, err := readProcFile("uptime"); err == nil {
				info.Uptime = parseUptime(raw)
			}
			if raw, err := readProcFile("loadavg"); err == nil {
				info.LoadAvg = parseLoadAvg(raw)
			}
			if raw, err := readProcFile("cpuinfo"); err == nil {
				info.CPUCount = parseCpuCount(raw)
			}
			if raw, err := readProcFile("meminfo"); err == nil {
				memTotal, memAvail, swapTotal, swapFree := parseMemInfo(raw)
				info.MemTotalMB = memTotal / 1024
				info.MemAvailMB = memAvail / 1024
				info.SwapTotalMB = swapTotal / 1024
				info.SwapUsedMB = (swapTotal - swapFree) / 1024
			}
			return info, nil
		},
	)

	investigatePort := handler.TypedHandler[InvestigatePortInput, InvestigatePortOutput](
		"investigate_port",
		"Investigate a TCP port: find the listening process, show its tree, check systemd unit status, and fetch recent logs. Single tool replaces port_list + ps_list + systemd_status + systemd_logs.",
		func(_ context.Context, input InvestigatePortInput) (InvestigatePortOutput, error) {
			if input.Port <= 0 || input.Port > 65535 {
				return InvestigatePortOutput{}, fmt.Errorf("[%s] port must be 1-65535", handler.ErrInvalidParam)
			}
			logLines := input.LogLines
			if logLines <= 0 {
				logLines = 20
			}

			result := InvestigatePortOutput{Port: input.Port}
			ssOut, _, _ := runCmd("ss", "-tlnp", fmt.Sprintf("sport = :%d", input.Port))
			var pid int
			for line := range strings.SplitSeq(ssOut, "\n") {
				if !strings.Contains(line, "LISTEN") {
					continue
				}
				if p := extractPidFromSsLine(line); p > 0 {
					pid = p
					break
				}
			}
			if pid == 0 {
				return result, fmt.Errorf("[%s] no process listening on port %d", handler.ErrNotFound, input.Port)
			}

			psOut, _, _ := runCmd("ps", "-p", strconv.Itoa(pid), "-o", "user,pid,pcpu,pmem,vsz,rss,tty,stat,start,time,command", "--no-headers")
			if psOut != "" {
				if p, ok := parsePsLine(psOut); ok {
					result.Process = &p
				}
			}

			treeOut, _, err := runCmd("pstree", "-p", strconv.Itoa(pid))
			if err == nil {
				result.Tree = treeOut
			}

			unitOut, _, _ := runCmd("systemctl", "--user", "status", strconv.Itoa(pid))
			if unitOut != "" {
				result.SystemdUnit = extractSystemdUnit(unitOut)
				result.SystemdStatus = unitOut
			}
			if result.SystemdUnit != "" && isValidUnitName(result.SystemdUnit) {
				logsOut, _, _ := runCmd("journalctl", "--user-unit", result.SystemdUnit, "-n", strconv.Itoa(logLines), "--no-pager")
				result.RecentLogs = logsOut
			}

			return result, nil
		},
	)
	investigatePort.MaxResultChars = 9000
	investigatePort.SearchTerms = []string{"debug port", "what is using this port", "port investigation"}

	investigateService := handler.TypedHandler[InvestigateServiceInput, InvestigateServiceOutput](
		"investigate_service",
		"Investigate a systemd service: get status, find its processes, check its ports, and fetch recent logs. Single tool replaces systemd_status + ps_list + port_list + systemd_logs.",
		func(_ context.Context, input InvestigateServiceInput) (InvestigateServiceOutput, error) {
			if input.Unit == "" {
				return InvestigateServiceOutput{}, fmt.Errorf("[%s] unit is required", handler.ErrInvalidParam)
			}
			if !isValidUnitName(input.Unit) {
				return InvestigateServiceOutput{}, fmt.Errorf("[%s] invalid unit name %q", handler.ErrInvalidParam, input.Unit)
			}
			logLines := input.LogLines
			if logLines <= 0 {
				logLines = 20
			}

			scope := "--user"
			journalFlag := "--user-unit"
			if input.System {
				scope = ""
				journalFlag = "-u"
			}

			result := InvestigateServiceOutput{Unit: input.Unit}
			var statusArgs []string
			if scope != "" {
				statusArgs = append(statusArgs, scope)
			}
			statusArgs = append(statusArgs, "show", "--property=ActiveState,SubState,MainPID", input.Unit)
			statusOut, _, _ := runCmd("systemctl", statusArgs...)
			props := parseSystemdProperties(statusOut)
			result.ActiveState = props["ActiveState"]
			result.SubState = props["SubState"]
			result.MainPID, _ = strconv.Atoi(props["MainPID"])

			if result.MainPID > 0 {
				psOut, _, _ := runCmd("ps", "-p", strconv.Itoa(result.MainPID), "-o", "user,pid,pcpu,pmem,vsz,rss,tty,stat,start,time,command", "--no-headers")
				if psOut != "" {
					if p, ok := parsePsLine(psOut); ok {
						result.Process = &p
					}
				}

				ssOut, _, _ := runCmd("ss", "-tlnp")
				result.Ports = filterSsPortsByPid(ssOut, strconv.Itoa(result.MainPID))
			}

			if result.Ports == nil {
				result.Ports = []PortEntry{}
			}

			logsOut, _, _ := runCmd("journalctl", journalFlag, input.Unit, "-n", strconv.Itoa(logLines), "--no-pager")
			result.RecentLogs = logsOut
			return result, nil
		},
	)
	investigateService.MaxResultChars = 9000
	investigateService.SearchTerms = []string{"debug service", "service investigation", "why is my service failing"}

	return []registry.ToolDefinition{
		psList,
		psTree,
		killProcess,
		portList,
		gpuStatus,
		systemInfo,
		investigatePort,
		investigateService,
	}
}

// ---------------------------------------------------------------------------
// main
// ---------------------------------------------------------------------------

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})).With("service", "process-mcp"))

	slog.Info("server starting", "name", "process-mcp", "version", "1.0.0")

	reg := registry.NewToolRegistry(registry.Config{
		Middleware: []registry.Middleware{
			registry.AuditMiddleware(""),
			registry.SafetyTierMiddleware(),
		},
	})
	mod := &ProcessModule{}
	reg.RegisterModule(mod)
	slog.Info("tools registered", "module", mod.Name(), "count", len(mod.Tools()))

	s := registry.NewMCPServer("process-mcp", "1.0.0")
	reg.RegisterWithServer(s)
	buildProcessResourceRegistry().RegisterWithServer(s)
	buildProcessPromptRegistry().RegisterWithServer(s)

	if err := registry.ServeAuto(s); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

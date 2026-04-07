package main

import (
	"regexp"
	"strconv"
	"strings"
)

// validUnitNameRe matches systemd unit names: alphanumeric, @, ., _, :, -.
var validUnitNameRe = regexp.MustCompile(`^[a-zA-Z0-9@._:\-]+$`)

// isValidUnitName returns true if name looks like a valid systemd unit name.
// Used to prevent argument injection when passing unit names to shell commands.
func isValidUnitName(name string) bool {
	return name != "" && len(name) <= 256 && validUnitNameRe.MatchString(name)
}

// ---------------------------------------------------------------------------
// Parsing helpers — extracted from handler closures for testability.
// ---------------------------------------------------------------------------

// parsePsLine parses a single line of `ps aux` output into a ProcessInfo.
// Returns ok=false if the line has fewer than 11 fields.
func parsePsLine(line string) (ProcessInfo, bool) {
	fields := strings.Fields(line)
	if len(fields) < 11 {
		return ProcessInfo{}, false
	}

	pid, _ := strconv.Atoi(fields[1])
	cpu, _ := strconv.ParseFloat(fields[2], 64)
	mem, _ := strconv.ParseFloat(fields[3], 64)
	vsz, _ := strconv.Atoi(fields[4])
	rss, _ := strconv.Atoi(fields[5])

	return ProcessInfo{
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
		Command: strings.Join(fields[10:], " "),
	}, true
}

// parseSsLines parses the output of `ss -tlnp` into a slice of PortEntry.
// filterPort, if > 0, restricts results to that port number.
func parseSsLines(output string, filterPort int) []PortEntry {
	lines := strings.Split(output, "\n")
	var ports []PortEntry
	for i, line := range lines {
		if i < 1 {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 || fields[0] != "LISTEN" {
			continue
		}

		localAddr := fields[3]
		addr, portNum, ok := parseAddrPort(localAddr)
		if !ok {
			continue
		}
		if filterPort > 0 && portNum != filterPort {
			continue
		}

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
	return ports
}

// parseAddrPort extracts address and port from an "addr:port" string.
// Uses the last colon as delimiter to handle IPv6 addresses.
func parseAddrPort(localAddr string) (addr string, port int, ok bool) {
	lastColon := strings.LastIndex(localAddr, ":")
	if lastColon < 0 {
		return "", 0, false
	}
	addr = localAddr[:lastColon]
	port, err := strconv.Atoi(localAddr[lastColon+1:])
	if err != nil {
		return "", 0, false
	}
	return addr, port, true
}

// extractPidFromSsLine extracts a PID from an ss output line containing "pid=N".
// Returns 0 if no PID is found.
func extractPidFromSsLine(line string) int {
	if _, after, ok := strings.Cut(line, "pid="); ok {
		if end := strings.IndexAny(after, ",)"); end > 0 {
			pid, _ := strconv.Atoi(after[:end])
			return pid
		}
		// No delimiter found — try the whole remaining string
		pid, _ := strconv.Atoi(strings.TrimSpace(after))
		return pid
	}
	return 0
}

// parseGpuInfo parses comma-separated nvidia-smi --query-gpu output into GpuInfo.
// Returns nil if fewer than 7 fields are present.
func parseGpuInfo(csvLine string) *GpuInfo {
	fields := strings.Split(csvLine, ", ")
	if len(fields) < 7 {
		return nil
	}
	temp, _ := strconv.Atoi(strings.TrimSpace(fields[2]))
	util, _ := strconv.Atoi(strings.TrimSpace(fields[3]))
	memUsed, _ := strconv.Atoi(strings.TrimSpace(fields[4]))
	memTotal, _ := strconv.Atoi(strings.TrimSpace(fields[5]))
	power, _ := strconv.ParseFloat(strings.TrimSpace(fields[6]), 64)

	return &GpuInfo{
		DriverVersion: strings.TrimSpace(fields[0]),
		Name:          strings.TrimSpace(fields[1]),
		Temperature:   temp,
		Utilization:   util,
		MemoryUsed:    memUsed,
		MemoryTotal:   memTotal,
		PowerDraw:     power,
	}
}

// parseGpuProcessLine parses a single line of nvidia-smi --query-compute-apps output.
// Returns ok=false if fewer than 3 comma-separated fields.
func parseGpuProcessLine(line string) (GpuProcess, bool) {
	pfields := strings.Split(line, ", ")
	if len(pfields) < 3 {
		return GpuProcess{}, false
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(pfields[0]))
	mem, _ := strconv.Atoi(strings.TrimSpace(pfields[2]))
	return GpuProcess{
		PID:        pid,
		Name:       strings.TrimSpace(pfields[1]),
		MemoryUsed: mem,
	}, true
}

// parseKernelVersion extracts the kernel version from /proc/version content.
// Returns empty string if fewer than 3 fields.
func parseKernelVersion(procVersion string) string {
	fields := strings.Fields(procVersion)
	if len(fields) >= 3 {
		return fields[2]
	}
	return ""
}

// parseUptime parses /proc/uptime content into a human-readable string.
// Returns empty string if the content is empty or unparseable.
func parseUptime(procUptime string) string {
	fields := strings.Fields(procUptime)
	if len(fields) < 1 {
		return ""
	}
	secs, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return ""
	}
	totalSecs := int(secs)
	days := totalSecs / 86400
	hours := (totalSecs % 86400) / 3600
	mins := (totalSecs % 3600) / 60
	return strings.Join([]string{
		strconv.Itoa(days) + "d",
		strconv.Itoa(hours) + "h",
		strconv.Itoa(mins) + "m",
	}, " ")
}

// parseLoadAvg parses /proc/loadavg content into 3 float64 values.
// Returns nil if fewer than 3 fields.
func parseLoadAvg(procLoadAvg string) []float64 {
	fields := strings.Fields(procLoadAvg)
	if len(fields) < 3 {
		return nil
	}
	avg := make([]float64, 3)
	avg[0], _ = strconv.ParseFloat(fields[0], 64)
	avg[1], _ = strconv.ParseFloat(fields[1], 64)
	avg[2], _ = strconv.ParseFloat(fields[2], 64)
	return avg
}

// parseCpuCount counts "processor" lines in /proc/cpuinfo content.
func parseCpuCount(procCpuinfo string) int {
	count := 0
	for line := range strings.SplitSeq(procCpuinfo, "\n") {
		if strings.HasPrefix(line, "processor") {
			count++
		}
	}
	return count
}

// parseMemInfo parses /proc/meminfo content and returns MemTotal, MemAvailable,
// SwapTotal, and SwapFree in kilobytes.
func parseMemInfo(procMeminfo string) (memTotal, memAvail, swapTotal, swapFree int) {
	memMap := make(map[string]int)
	for line := range strings.SplitSeq(procMeminfo, "\n") {
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
	return memMap["MemTotal"], memMap["MemAvailable"], memMap["SwapTotal"], memMap["SwapFree"]
}

// filterSsPortsByPid scans ss -tlnp output for LISTEN lines that contain
// the given PID string, and returns matching PortEntry items.
func filterSsPortsByPid(ssOutput string, pidStr string) []PortEntry {
	var ports []PortEntry
	for line := range strings.SplitSeq(ssOutput, "\n") {
		if !strings.Contains(line, pidStr) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 || fields[0] != "LISTEN" {
			continue
		}
		addr, portNum, ok := parseAddrPort(fields[3])
		if !ok {
			continue
		}
		ports = append(ports, PortEntry{
			Protocol: "tcp",
			Address:  addr,
			Port:     portNum,
		})
	}
	return ports
}

// extractSystemdUnit finds the first .service unit name from
// `systemctl status` output by scanning each line for words ending in ".service".
func extractSystemdUnit(statusOutput string) string {
	for line := range strings.SplitSeq(statusOutput, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasSuffix(line, ".service") || strings.Contains(line, ".service ") {
			parts := strings.FieldsSeq(line)
			for p := range parts {
				if strings.HasSuffix(p, ".service") {
					return p
				}
			}
		}
	}
	return ""
}

// parseSystemdProperties parses `systemctl show --property=...` output
// into key-value pairs.
func parseSystemdProperties(output string) map[string]string {
	props := make(map[string]string)
	for line := range strings.SplitSeq(output, "\n") {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			props[parts[0]] = parts[1]
		}
	}
	return props
}

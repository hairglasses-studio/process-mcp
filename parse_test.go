package main

import (
	"testing"
)

// ---------------------------------------------------------------------------
// isValidUnitName
// ---------------------------------------------------------------------------

func TestIsValidUnitName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"simple service", "nginx.service", true},
		{"timer unit", "backup.timer", true},
		{"instance unit", "container@myapp.service", true},
		{"dashed name", "foo-bar-baz.service", true},
		{"underscored name", "my_app.service", true},
		{"colon in name", "dbus-org.freedesktop.service", true},
		{"empty string", "", false},
		{"space in name", "my service.service", false},
		{"semicolon injection", "foo;rm -rf /", false},
		{"flag injection", "--user-unit=evil", false},
		{"command substitution", "$(whoami).service", false},
		{"backtick injection", "`id`.service", false},
		{"pipe injection", "foo|bar.service", false},
		{"newline injection", "foo\nbar.service", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isValidUnitName(tc.input)
			if got != tc.want {
				t.Errorf("isValidUnitName(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parsePsLine
// ---------------------------------------------------------------------------

func TestParsePsLine(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		wantOK  bool
		wantPID int
		wantCPU float64
		wantCmd string
	}{
		{
			name:    "normal line",
			line:    "root         1  0.0  0.1  21448 12884 ?        Ss   Apr01   0:03 /sbin/init",
			wantOK:  true,
			wantPID: 1,
			wantCPU: 0.0,
			wantCmd: "/sbin/init",
		},
		{
			name:    "multi-word command",
			line:    "hg        1234  5.2  1.3 456789 12345 pts/0    Sl+  10:30   0:15 /usr/bin/python3 -m http.server 8080",
			wantOK:  true,
			wantPID: 1234,
			wantCPU: 5.2,
			wantCmd: "/usr/bin/python3 -m http.server 8080",
		},
		{
			name:   "too few fields",
			line:   "root 1 0.0 0.1",
			wantOK: false,
		},
		{
			name:   "empty line",
			line:   "",
			wantOK: false,
		},
		{
			name:    "non-numeric PID",
			line:    "root    abc  0.0  0.1  21448 12884 ?        Ss   Apr01   0:03 /sbin/init",
			wantOK:  true,
			wantPID: 0,  // strconv.Atoi returns 0 for invalid input
			wantCPU: 0.0,
			wantCmd: "/sbin/init",
		},
		{
			name:    "non-numeric CPU",
			line:    "root         1  N/A  0.1  21448 12884 ?        Ss   Apr01   0:03 /sbin/init",
			wantOK:  true,
			wantPID: 1,
			wantCPU: 0.0, // parse error defaults to 0
			wantCmd: "/sbin/init",
		},
		{
			name:    "exactly 11 fields",
			line:    "hg 99 1.0 2.0 100 200 pts/1 S+ 09:00 0:01 bash",
			wantOK:  true,
			wantPID: 99,
			wantCPU: 1.0,
			wantCmd: "bash",
		},
		{
			name:   "10 fields — just under threshold",
			line:   "hg 99 1.0 2.0 100 200 pts/1 S+ 09:00 0:01",
			wantOK: false,
		},
		{
			name:    "high CPU/mem values",
			line:    "hg 42 100.0 99.9 9999999 8888888 ?  R  Apr01 99:59 stress --cpu 16",
			wantOK:  true,
			wantPID: 42,
			wantCPU: 100.0,
			wantCmd: "stress --cpu 16",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parsePsLine(tc.line)
			if ok != tc.wantOK {
				t.Fatalf("parsePsLine ok=%v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if got.PID != tc.wantPID {
				t.Errorf("PID=%d, want %d", got.PID, tc.wantPID)
			}
			if got.CPU != tc.wantCPU {
				t.Errorf("CPU=%f, want %f", got.CPU, tc.wantCPU)
			}
			if got.Command != tc.wantCmd {
				t.Errorf("Command=%q, want %q", got.Command, tc.wantCmd)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseAddrPort
// ---------------------------------------------------------------------------

func TestParseAddrPort(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantAddr string
		wantPort int
		wantOK   bool
	}{
		{
			name:     "IPv4 standard",
			input:    "127.0.0.1:8080",
			wantAddr: "127.0.0.1",
			wantPort: 8080,
			wantOK:   true,
		},
		{
			name:     "IPv4 wildcard",
			input:    "0.0.0.0:443",
			wantAddr: "0.0.0.0",
			wantPort: 443,
			wantOK:   true,
		},
		{
			name:     "IPv4 star wildcard",
			input:    "*:22",
			wantAddr: "*",
			wantPort: 22,
			wantOK:   true,
		},
		{
			name:     "IPv6 loopback",
			input:    "::1:3000",
			wantAddr: "::1",
			wantPort: 3000,
			wantOK:   true,
		},
		{
			name:     "IPv6 wildcard",
			input:    ":::8080",
			wantAddr: "::",
			wantPort: 8080,
			wantOK:   true,
		},
		{
			name:   "no colon",
			input:  "localhost",
			wantOK: false,
		},
		{
			name:   "non-numeric port",
			input:  "127.0.0.1:http",
			wantOK: false,
		},
		{
			name:   "empty string",
			input:  "",
			wantOK: false,
		},
		{
			name:     "port 0",
			input:    "127.0.0.1:0",
			wantAddr: "127.0.0.1",
			wantPort: 0,
			wantOK:   true,
		},
		{
			name:     "high port",
			input:    "10.0.0.1:65535",
			wantAddr: "10.0.0.1",
			wantPort: 65535,
			wantOK:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			addr, port, ok := parseAddrPort(tc.input)
			if ok != tc.wantOK {
				t.Fatalf("ok=%v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if addr != tc.wantAddr {
				t.Errorf("addr=%q, want %q", addr, tc.wantAddr)
			}
			if port != tc.wantPort {
				t.Errorf("port=%d, want %d", port, tc.wantPort)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// extractPidFromSsLine
// ---------------------------------------------------------------------------

func TestExtractPidFromSsLine(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		wantPID int
	}{
		{
			name:    "standard ss output",
			line:    `LISTEN 0      128    127.0.0.1:8080  0.0.0.0:*  users:(("node",pid=1234,fd=19))`,
			wantPID: 1234,
		},
		{
			name:    "multiple users entries",
			line:    `LISTEN 0      128    *:22  *:*  users:(("sshd",pid=567,fd=3),("sshd",pid=568,fd=3))`,
			wantPID: 567,
		},
		{
			name:    "no pid field",
			line:    `LISTEN 0      128    *:80  *:*  users:(("nginx",fd=6))`,
			wantPID: 0,
		},
		{
			name:    "empty line",
			line:    "",
			wantPID: 0,
		},
		{
			name:    "header line",
			line:    "State  Recv-Q Send-Q  Local Address:Port  Peer Address:Port Process",
			wantPID: 0,
		},
		{
			name:    "pid at end of string with paren",
			line:    `users:(("app",pid=9999))`,
			wantPID: 9999,
		},
		{
			name:    "pid followed by comma",
			line:    `users:(("app",pid=42,fd=3))`,
			wantPID: 42,
		},
		{
			name:    "non-numeric pid value",
			line:    `users:(("app",pid=abc,fd=3))`,
			wantPID: 0,
		},
		{
			name:    "pid= with no value before delimiter",
			line:    `users:(("app",pid=,fd=3))`,
			wantPID: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractPidFromSsLine(tc.line)
			if got != tc.wantPID {
				t.Errorf("extractPidFromSsLine=%d, want %d", got, tc.wantPID)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseSsLines
// ---------------------------------------------------------------------------

func TestParseSsLines(t *testing.T) {
	tests := []struct {
		name       string
		output     string
		filterPort int
		wantCount  int
		wantPort   int
	}{
		{
			name: "typical ss output",
			output: `State  Recv-Q Send-Q  Local Address:Port  Peer Address:Port Process
LISTEN 0      128    127.0.0.1:8080  0.0.0.0:*  users:(("node",pid=1234,fd=19))
LISTEN 0      128    0.0.0.0:22  0.0.0.0:*  users:(("sshd",pid=567,fd=3))`,
			filterPort: 0,
			wantCount:  2,
		},
		{
			name: "filter by port",
			output: `State  Recv-Q Send-Q  Local Address:Port  Peer Address:Port Process
LISTEN 0      128    127.0.0.1:8080  0.0.0.0:*  users:(("node",pid=1234,fd=19))
LISTEN 0      128    0.0.0.0:22  0.0.0.0:*  users:(("sshd",pid=567,fd=3))`,
			filterPort: 22,
			wantCount:  1,
			wantPort:   22,
		},
		{
			name: "filter by port — no match",
			output: `State  Recv-Q Send-Q  Local Address:Port  Peer Address:Port Process
LISTEN 0      128    127.0.0.1:8080  0.0.0.0:*  users:(("node",pid=1234,fd=19))`,
			filterPort: 9999,
			wantCount:  0,
		},
		{
			name:       "empty output",
			output:     "",
			filterPort: 0,
			wantCount:  0,
		},
		{
			name:       "header only",
			output:     "State  Recv-Q Send-Q  Local Address:Port  Peer Address:Port Process",
			filterPort: 0,
			wantCount:  0,
		},
		{
			name: "non-LISTEN state skipped",
			output: `State  Recv-Q Send-Q  Local Address:Port  Peer Address:Port Process
ESTAB  0      0     10.0.0.1:8080  10.0.0.2:5432`,
			filterPort: 0,
			wantCount:  0,
		},
		{
			name: "too few fields skipped",
			output: `header
LISTEN 0 128`,
			filterPort: 0,
			wantCount:  0,
		},
		{
			name: "malformed port in address skipped",
			output: `State  Recv-Q Send-Q  Local Address:Port  Peer Address:Port Process
LISTEN 0      128    127.0.0.1:abc  0.0.0.0:*  users:(("node",pid=1234,fd=19))`,
			filterPort: 0,
			wantCount:  0,
		},
		{
			name: "no colon in address skipped",
			output: `State  Recv-Q Send-Q  Local Address:Port  Peer Address:Port Process
LISTEN 0      128    localhost  0.0.0.0:*  users:(("node",pid=1234,fd=19))`,
			filterPort: 0,
			wantCount:  0,
		},
		{
			name: "users field extracted",
			output: `State  Recv-Q Send-Q  Local Address:Port  Peer Address:Port Process
LISTEN 0      128    *:80  *:*  users:(("nginx",pid=100,fd=6))`,
			filterPort: 0,
			wantCount:  1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ports := parseSsLines(tc.output, tc.filterPort)
			if len(ports) != tc.wantCount {
				t.Fatalf("got %d ports, want %d", len(ports), tc.wantCount)
			}
			if tc.wantPort > 0 && len(ports) > 0 && ports[0].Port != tc.wantPort {
				t.Errorf("port=%d, want %d", ports[0].Port, tc.wantPort)
			}
			// Verify all entries are TCP protocol
			for _, p := range ports {
				if p.Protocol != "tcp" {
					t.Errorf("protocol=%q, want tcp", p.Protocol)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseGpuInfo
// ---------------------------------------------------------------------------

func TestParseGpuInfo(t *testing.T) {
	tests := []struct {
		name      string
		csvLine   string
		wantNil   bool
		wantName  string
		wantTemp  int
		wantMem   int
		wantPower float64
	}{
		{
			name:      "normal output",
			csvLine:   "550.120, NVIDIA GeForce RTX 3080, 45, 12, 1024, 10240, 120.50",
			wantNil:   false,
			wantName:  "NVIDIA GeForce RTX 3080",
			wantTemp:  45,
			wantMem:   10240,
			wantPower: 120.50,
		},
		{
			name:    "too few fields",
			csvLine: "550.120, NVIDIA GeForce RTX 3080, 45",
			wantNil: true,
		},
		{
			name:    "empty string",
			csvLine: "",
			wantNil: true,
		},
		{
			name:      "non-numeric temperature",
			csvLine:   "550.120, RTX 3080, N/A, 12, 1024, 10240, 120.50",
			wantNil:   false,
			wantName:  "RTX 3080",
			wantTemp:  0, // parse error defaults to 0
			wantMem:   10240,
			wantPower: 120.50,
		},
		{
			name:      "non-numeric power",
			csvLine:   "550.120, RTX 3080, 45, 12, 1024, 10240, [Not Supported]",
			wantNil:   false,
			wantTemp:  45,
			wantPower: 0.0,
		},
		{
			name:      "extra fields ignored",
			csvLine:   "550.120, RTX 3080, 45, 12, 1024, 10240, 120.50, extra1, extra2",
			wantNil:   false,
			wantTemp:  45,
			wantPower: 120.50,
		},
		{
			name:      "whitespace in fields",
			csvLine:   "  550.120  ,  RTX 3080  ,  45  ,  12  ,  1024  ,  10240  ,  120.50  ",
			wantNil:   false,
			wantName:  "RTX 3080",
			wantTemp:  45,
			wantPower: 120.50,
		},
		{
			name:    "six fields — boundary",
			csvLine: "550.120, RTX 3080, 45, 12, 1024, 10240",
			wantNil: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseGpuInfo(tc.csvLine)
			if tc.wantNil {
				if got != nil {
					t.Fatalf("expected nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil GpuInfo")
			}
			if tc.wantName != "" && got.Name != tc.wantName {
				t.Errorf("Name=%q, want %q", got.Name, tc.wantName)
			}
			if got.Temperature != tc.wantTemp {
				t.Errorf("Temperature=%d, want %d", got.Temperature, tc.wantTemp)
			}
			if tc.wantMem > 0 && got.MemoryTotal != tc.wantMem {
				t.Errorf("MemoryTotal=%d, want %d", got.MemoryTotal, tc.wantMem)
			}
			if got.PowerDraw != tc.wantPower {
				t.Errorf("PowerDraw=%f, want %f", got.PowerDraw, tc.wantPower)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseGpuProcessLine
// ---------------------------------------------------------------------------

func TestParseGpuProcessLine(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		wantOK  bool
		wantPID int
		wantMem int
	}{
		{
			name:    "normal line",
			line:    "1234, python3, 2048",
			wantOK:  true,
			wantPID: 1234,
			wantMem: 2048,
		},
		{
			name:   "too few fields",
			line:   "1234, python3",
			wantOK: false,
		},
		{
			name:   "empty string",
			line:   "",
			wantOK: false,
		},
		{
			name:    "non-numeric PID",
			line:    "abc, python3, 2048",
			wantOK:  true,
			wantPID: 0,
			wantMem: 2048,
		},
		{
			name:    "non-numeric memory",
			line:    "1234, python3, N/A",
			wantOK:  true,
			wantPID: 1234,
			wantMem: 0,
		},
		{
			name:    "whitespace in fields",
			line:    "  5678  ,  /usr/bin/Xwayland  ,  512  ",
			wantOK:  true,
			wantPID: 5678,
			wantMem: 512,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseGpuProcessLine(tc.line)
			if ok != tc.wantOK {
				t.Fatalf("ok=%v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if got.PID != tc.wantPID {
				t.Errorf("PID=%d, want %d", got.PID, tc.wantPID)
			}
			if got.MemoryUsed != tc.wantMem {
				t.Errorf("MemoryUsed=%d, want %d", got.MemoryUsed, tc.wantMem)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseKernelVersion
// ---------------------------------------------------------------------------

func TestParseKernelVersion(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "standard /proc/version",
			in:   "Linux version 6.12.77-1-MANJARO (builduser@buildhost) (gcc 14.2.1) #1 SMP PREEMPT_DYNAMIC",
			want: "6.12.77-1-MANJARO",
		},
		{
			name: "minimal 3 fields",
			in:   "Linux version 5.15.0",
			want: "5.15.0",
		},
		{
			name: "two fields — too short",
			in:   "Linux version",
			want: "",
		},
		{
			name: "empty",
			in:   "",
			want: "",
		},
		{
			name: "single field",
			in:   "Linux",
			want: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseKernelVersion(tc.in)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseUptime
// ---------------------------------------------------------------------------

func TestParseUptime(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "normal uptime",
			in:   "123456.78 234567.89",
			want: "1d 10h 17m",
		},
		{
			name: "zero uptime",
			in:   "0.00 0.00",
			want: "0d 0h 0m",
		},
		{
			name: "less than a day",
			in:   "3723.45 1000.00",
			want: "0d 1h 2m",
		},
		{
			name: "empty string",
			in:   "",
			want: "",
		},
		{
			name: "non-numeric",
			in:   "abc def",
			want: "",
		},
		{
			name: "single value",
			in:   "86400.00",
			want: "1d 0h 0m",
		},
		{
			name: "large uptime — 30 days",
			in:   "2592000.00 1000.00",
			want: "30d 0h 0m",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseUptime(tc.in)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseLoadAvg
// ---------------------------------------------------------------------------

func TestParseLoadAvg(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantNil bool
		wantLen int
		want0   float64
	}{
		{
			name:    "normal loadavg",
			in:      "1.23 4.56 7.89 2/1234 56789",
			wantLen: 3,
			want0:   1.23,
		},
		{
			name:    "integer values",
			in:      "0 0 0 1/100 1234",
			wantLen: 3,
			want0:   0.0,
		},
		{
			name:    "two fields — too short",
			in:      "1.23 4.56",
			wantNil: true,
		},
		{
			name:    "empty",
			in:      "",
			wantNil: true,
		},
		{
			name:    "high load",
			in:      "64.00 48.00 32.00 128/2048 99999",
			wantLen: 3,
			want0:   64.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseLoadAvg(tc.in)
			if tc.wantNil {
				if got != nil {
					t.Fatalf("expected nil, got %v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil")
			}
			if len(got) != tc.wantLen {
				t.Fatalf("len=%d, want %d", len(got), tc.wantLen)
			}
			if got[0] != tc.want0 {
				t.Errorf("[0]=%f, want %f", got[0], tc.want0)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseCpuCount
// ---------------------------------------------------------------------------

func TestParseCpuCount(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		count int
	}{
		{
			name: "single CPU",
			in: `processor	: 0
vendor_id	: GenuineIntel
model name	: Intel Core i7`,
			count: 1,
		},
		{
			name: "four CPUs",
			in: `processor	: 0
model name	: AMD Ryzen

processor	: 1
model name	: AMD Ryzen

processor	: 2
model name	: AMD Ryzen

processor	: 3
model name	: AMD Ryzen`,
			count: 4,
		},
		{
			name:  "empty",
			in:    "",
			count: 0,
		},
		{
			name: "no processor lines",
			in: `vendor_id	: GenuineIntel
model name	: Intel Core i7`,
			count: 0,
		},
		{
			name: "processor in model name — should not count",
			in: `processor	: 0
model name	: processorfoo
other processor line`,
			count: 1, // only the line starting with "processor" counts
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseCpuCount(tc.in)
			if got != tc.count {
				t.Errorf("got %d, want %d", got, tc.count)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseMemInfo
// ---------------------------------------------------------------------------

func TestParseMemInfo(t *testing.T) {
	tests := []struct {
		name          string
		in            string
		wantTotal     int
		wantAvail     int
		wantSwapTotal int
		wantSwapFree  int
	}{
		{
			name: "standard meminfo",
			in: `MemTotal:       131891204 kB
MemFree:         1234567 kB
MemAvailable:   98765432 kB
Buffers:          123456 kB
Cached:          7654321 kB
SwapCached:            0 kB
SwapTotal:       8388604 kB
SwapFree:        8388604 kB`,
			wantTotal:     131891204,
			wantAvail:     98765432,
			wantSwapTotal: 8388604,
			wantSwapFree:  8388604,
		},
		{
			name:          "empty",
			in:            "",
			wantTotal:     0,
			wantAvail:     0,
			wantSwapTotal: 0,
			wantSwapFree:  0,
		},
		{
			name: "missing MemAvailable",
			in: `MemTotal:       131891204 kB
SwapTotal:       8388604 kB
SwapFree:        1000000 kB`,
			wantTotal:     131891204,
			wantAvail:     0,
			wantSwapTotal: 8388604,
			wantSwapFree:  1000000,
		},
		{
			name: "missing swap lines",
			in: `MemTotal:       131891204 kB
MemAvailable:   98765432 kB`,
			wantTotal:     131891204,
			wantAvail:     98765432,
			wantSwapTotal: 0,
			wantSwapFree:  0,
		},
		{
			name: "malformed value",
			in: `MemTotal:       abc kB
MemAvailable:   98765432 kB`,
			wantTotal: 0, // parse error defaults to 0
			wantAvail: 98765432,
		},
		{
			name: "line with only key — no value",
			in:   `MemTotal:`,
			// Only 1 field after split, so the len(parts) >= 2 check fails
			wantTotal: 0,
		},
		{
			name: "extra lines mixed in",
			in: `MemTotal:       100000 kB
RandomLine:     123456 kB
MemAvailable:   50000 kB
AnotherRandom:  789 kB
SwapTotal:      20000 kB
SwapFree:       10000 kB`,
			wantTotal:     100000,
			wantAvail:     50000,
			wantSwapTotal: 20000,
			wantSwapFree:  10000,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			total, avail, swapTotal, swapFree := parseMemInfo(tc.in)
			if total != tc.wantTotal {
				t.Errorf("MemTotal=%d, want %d", total, tc.wantTotal)
			}
			if avail != tc.wantAvail {
				t.Errorf("MemAvailable=%d, want %d", avail, tc.wantAvail)
			}
			if swapTotal != tc.wantSwapTotal {
				t.Errorf("SwapTotal=%d, want %d", swapTotal, tc.wantSwapTotal)
			}
			if swapFree != tc.wantSwapFree {
				t.Errorf("SwapFree=%d, want %d", swapFree, tc.wantSwapFree)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// filterSsPortsByPid
// ---------------------------------------------------------------------------

func TestFilterSsPortsByPid(t *testing.T) {
	ssOutput := `State  Recv-Q Send-Q  Local Address:Port  Peer Address:Port Process
LISTEN 0      128    127.0.0.1:8080  0.0.0.0:*  users:(("node",pid=1234,fd=19))
LISTEN 0      128    0.0.0.0:22  0.0.0.0:*  users:(("sshd",pid=567,fd=3))
LISTEN 0      128    127.0.0.1:3000  0.0.0.0:*  users:(("node",pid=1234,fd=20))
ESTAB  0      0     10.0.0.1:8080  10.0.0.2:5432  users:(("node",pid=1234,fd=21))`

	tests := []struct {
		name      string
		pidStr    string
		wantCount int
		wantPorts []int
	}{
		{
			name:      "match PID 1234 — two LISTEN entries",
			pidStr:    "1234",
			wantCount: 2,
			wantPorts: []int{8080, 3000},
		},
		{
			name:      "match PID 567 — one LISTEN entry",
			pidStr:    "567",
			wantCount: 1,
			wantPorts: []int{22},
		},
		{
			name:      "no match",
			pidStr:    "9999",
			wantCount: 0,
		},
		{
			name:      "empty PID",
			pidStr:    "",
			wantCount: 3, // empty string matches everything with LISTEN
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ports := filterSsPortsByPid(ssOutput, tc.pidStr)
			if len(ports) != tc.wantCount {
				t.Fatalf("got %d ports, want %d", len(ports), tc.wantCount)
			}
			for i, wantPort := range tc.wantPorts {
				if i < len(ports) && ports[i].Port != wantPort {
					t.Errorf("port[%d]=%d, want %d", i, ports[i].Port, wantPort)
				}
			}
			for _, p := range ports {
				if p.Protocol != "tcp" {
					t.Errorf("protocol=%q, want tcp", p.Protocol)
				}
			}
		})
	}
}

func TestFilterSsPortsByPid_EmptyOutput(t *testing.T) {
	ports := filterSsPortsByPid("", "1234")
	if len(ports) != 0 {
		t.Errorf("expected 0 ports, got %d", len(ports))
	}
}

func TestFilterSsPortsByPid_MalformedAddr(t *testing.T) {
	ssOutput := `header
LISTEN 0      128    noport  0.0.0.0:*  users:(("app",pid=100,fd=3))`
	ports := filterSsPortsByPid(ssOutput, "100")
	if len(ports) != 0 {
		t.Errorf("expected 0 ports (malformed addr), got %d", len(ports))
	}
}

func TestFilterSsPortsByPid_TooFewFields(t *testing.T) {
	ssOutput := `header
LISTEN 0      128`
	ports := filterSsPortsByPid(ssOutput, "128")
	if len(ports) != 0 {
		t.Errorf("expected 0 ports (too few fields), got %d", len(ports))
	}
}

// ---------------------------------------------------------------------------
// extractSystemdUnit
// ---------------------------------------------------------------------------

func TestExtractSystemdUnit(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "standard systemctl status output",
			in: `● process-mcp.service - Process MCP Server
     Loaded: loaded (/home/hg/.config/systemd/user/process-mcp.service; enabled)
     Active: active (running) since Sat 2026-04-05 10:00:00 CEST; 1h ago
   Main PID: 1234 (process-mcp)`,
			want: "process-mcp.service",
		},
		{
			name: "service name at end of line",
			in:   "foo-bar.service",
			want: "foo-bar.service",
		},
		{
			name: "service name in middle of line",
			in:   "  ● my-app.service - My Application",
			want: "my-app.service",
		},
		{
			name: "no service in output",
			in: `  Main PID: 1234 (process-mcp)
     Tasks: 5 (limit: 38400)`,
			want: "",
		},
		{
			name: "empty string",
			in:   "",
			want: "",
		},
		{
			name: "multiple services — returns first",
			in: `  ● first.service - First
  ● second.service - Second`,
			want: "first.service",
		},
		{
			name: "service with @ instance",
			in:   "  ● container@myapp.service - Container for myapp",
			want: "container@myapp.service",
		},
		{
			name: ".service as standalone word is matched",
			in:   "This is about service management, not a .service file",
			want: ".service",
		},
		{
			name: "service substring without .service suffix",
			in:   "This is about service management only",
			want: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractSystemdUnit(tc.in)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseSystemdProperties
// ---------------------------------------------------------------------------

func TestParseSystemdProperties(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want map[string]string
	}{
		{
			name: "standard properties",
			in:   "ActiveState=active\nSubState=running\nMainPID=1234",
			want: map[string]string{
				"ActiveState": "active",
				"SubState":    "running",
				"MainPID":     "1234",
			},
		},
		{
			name: "empty",
			in:   "",
			want: map[string]string{},
		},
		{
			name: "value with equals sign",
			in:   "ExecStart=/usr/bin/foo --arg=bar",
			want: map[string]string{
				"ExecStart": "/usr/bin/foo --arg=bar",
			},
		},
		{
			name: "no equals sign — line skipped",
			in:   "InvalidLine\nActiveState=active",
			want: map[string]string{
				"ActiveState": "active",
			},
		},
		{
			name: "empty value",
			in:   "MainPID=0",
			want: map[string]string{
				"MainPID": "0",
			},
		},
		{
			name: "inactive service",
			in:   "ActiveState=inactive\nSubState=dead\nMainPID=0",
			want: map[string]string{
				"ActiveState": "inactive",
				"SubState":    "dead",
				"MainPID":     "0",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseSystemdProperties(tc.in)
			for k, wantV := range tc.want {
				if gotV, ok := got[k]; !ok {
					t.Errorf("missing key %q", k)
				} else if gotV != wantV {
					t.Errorf("%s=%q, want %q", k, gotV, wantV)
				}
			}
			// Verify no extra keys
			for k := range got {
				if _, ok := tc.want[k]; !ok {
					t.Errorf("unexpected key %q=%q", k, got[k])
				}
			}
		})
	}
}

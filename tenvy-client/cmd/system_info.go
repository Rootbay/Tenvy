package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	gnet "github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

type SystemInfoCommandPayload struct {
	Refresh bool `json:"refresh,omitempty"`
}

type SystemInfoReport struct {
	CollectedAt string                       `json:"collectedAt"`
	Host        SystemInfoHost               `json:"host"`
	OS          SystemInfoOS                 `json:"os"`
	Hardware    SystemInfoHardware           `json:"hardware"`
	Memory      SystemInfoMemory             `json:"memory"`
	Storage     []SystemInfoStorage          `json:"storage,omitempty"`
	Network     []SystemInfoNetworkInterface `json:"network,omitempty"`
	Runtime     SystemInfoRuntime            `json:"runtime"`
	Environment SystemInfoEnvironment        `json:"environment"`
	Agent       SystemInfoAgent              `json:"agent"`
	Warnings    []string                     `json:"warnings,omitempty"`
}

type SystemInfoHost struct {
	Hostname      string `json:"hostname,omitempty"`
	HostID        string `json:"hostId,omitempty"`
	Domain        string `json:"domain,omitempty"`
	IPAddress     string `json:"ipAddress,omitempty"`
	UptimeSeconds uint64 `json:"uptimeSeconds,omitempty"`
	BootTime      string `json:"bootTime,omitempty"`
	Timezone      string `json:"timezone,omitempty"`
}

type SystemInfoOS struct {
	Platform       string `json:"platform,omitempty"`
	Family         string `json:"family,omitempty"`
	Version        string `json:"version,omitempty"`
	KernelVersion  string `json:"kernelVersion,omitempty"`
	KernelArch     string `json:"kernelArch,omitempty"`
	Procs          uint64 `json:"procs,omitempty"`
	Virtualization string `json:"virtualization,omitempty"`
}

type SystemInfoHardware struct {
	Architecture         string          `json:"architecture"`
	VirtualizationRole   string          `json:"virtualizationRole,omitempty"`
	VirtualizationSystem string          `json:"virtualizationSystem,omitempty"`
	PhysicalCores        int             `json:"physicalCores,omitempty"`
	LogicalCores         int             `json:"logicalCores,omitempty"`
	CPUs                 []SystemInfoCPU `json:"cpus,omitempty"`
}

type SystemInfoCPU struct {
	ID        int     `json:"id"`
	Model     string  `json:"model,omitempty"`
	Vendor    string  `json:"vendor,omitempty"`
	Family    string  `json:"family,omitempty"`
	CacheSize uint32  `json:"cacheSizeKb,omitempty"`
	Mhz       float64 `json:"mhz,omitempty"`
	Stepping  int32   `json:"stepping,omitempty"`
	Microcode string  `json:"microcode,omitempty"`
	Cores     int32   `json:"cores,omitempty"`
}

type SystemInfoMemory struct {
	TotalBytes      uint64  `json:"totalBytes,omitempty"`
	AvailableBytes  uint64  `json:"availableBytes,omitempty"`
	UsedBytes       uint64  `json:"usedBytes,omitempty"`
	UsedPercent     float64 `json:"usedPercent,omitempty"`
	SwapTotalBytes  uint64  `json:"swapTotalBytes,omitempty"`
	SwapFreeBytes   uint64  `json:"swapFreeBytes,omitempty"`
	SwapUsedBytes   uint64  `json:"swapUsedBytes,omitempty"`
	SwapUsedPercent float64 `json:"swapUsedPercent,omitempty"`
}

type SystemInfoStorage struct {
	Device      string  `json:"device"`
	Mountpoint  string  `json:"mountpoint"`
	Filesystem  string  `json:"filesystem,omitempty"`
	TotalBytes  uint64  `json:"totalBytes,omitempty"`
	UsedBytes   uint64  `json:"usedBytes,omitempty"`
	FreeBytes   uint64  `json:"freeBytes,omitempty"`
	UsedPercent float64 `json:"usedPercent,omitempty"`
	ReadOnly    bool    `json:"readOnly,omitempty"`
}

type SystemInfoNetworkInterface struct {
	Name      string                     `json:"name"`
	MTU       int                        `json:"mtu,omitempty"`
	Hardware  string                     `json:"macAddress,omitempty"`
	Flags     []string                   `json:"flags,omitempty"`
	Addresses []SystemInfoNetworkAddress `json:"addresses,omitempty"`
}

type SystemInfoNetworkAddress struct {
	Address string `json:"address"`
	Family  string `json:"family,omitempty"`
}

type SystemInfoRuntime struct {
	GoVersion   string            `json:"goVersion"`
	GoOS        string            `json:"goOs"`
	GoArch      string            `json:"goArch"`
	LogicalCPUs int               `json:"logicalCpus"`
	GoMaxProcs  int               `json:"goMaxProcs"`
	Goroutines  int               `json:"goroutines"`
	Process     SystemInfoProcess `json:"process"`
}

type SystemInfoProcess struct {
	PID              int     `json:"pid"`
	ParentPID        int     `json:"parentPid,omitempty"`
	Executable       string  `json:"executable,omitempty"`
	CommandLine      string  `json:"commandLine,omitempty"`
	WorkingDirectory string  `json:"workingDirectory,omitempty"`
	CreateTime       string  `json:"createTime,omitempty"`
	UptimeSeconds    float64 `json:"uptimeSeconds,omitempty"`
	CPUPercent       float64 `json:"cpuPercent,omitempty"`
	MemoryRSSBytes   uint64  `json:"memoryRssBytes,omitempty"`
	MemoryVMSBytes   uint64  `json:"memoryVmsBytes,omitempty"`
	NumThreads       int32   `json:"numThreads,omitempty"`
}

type SystemInfoEnvironment struct {
	Username         string   `json:"username,omitempty"`
	HomeDir          string   `json:"homeDir,omitempty"`
	Shell            string   `json:"shell,omitempty"`
	Lang             string   `json:"lang,omitempty"`
	PathSeparator    string   `json:"pathSeparator"`
	PathEntries      []string `json:"pathEntries,omitempty"`
	TempDir          string   `json:"tempDir,omitempty"`
	EnvironmentCount int      `json:"environmentCount,omitempty"`
}

type SystemInfoAgent struct {
	ID            string   `json:"id,omitempty"`
	Version       string   `json:"version,omitempty"`
	StartTime     string   `json:"startTime,omitempty"`
	UptimeSeconds uint64   `json:"uptimeSeconds,omitempty"`
	Tags          []string `json:"tags,omitempty"`
}

type SystemInfoCollector struct {
	agent       *Agent
	mu          sync.Mutex
	cache       string
	cacheExpiry time.Time
	cacheTTL    time.Duration
}

func NewSystemInfoCollector(agent *Agent) *SystemInfoCollector {
	return &SystemInfoCollector{
		agent:    agent,
		cacheTTL: 10 * time.Second,
	}
}

func (c *SystemInfoCollector) HandleCommand(ctx context.Context, cmd Command) CommandResult {
	result := CommandResult{
		CommandID:   cmd.ID,
		CompletedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}

	if ctx.Err() != nil {
		result.Success = false
		result.Error = ctx.Err().Error()
		return result
	}

	var payload SystemInfoCommandPayload
	if len(cmd.Payload) > 0 {
		if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("invalid system info payload: %v", err)
			return result
		}
	}

	snapshot, err := c.snapshot(ctx, payload.Refresh)
	if err != nil {
		result.Success = false
		result.Error = err.Error()
		return result
	}

	result.Success = true
	result.Output = snapshot
	return result
}

func (c *SystemInfoCollector) snapshot(ctx context.Context, refresh bool) (string, error) {
	c.mu.Lock()
	if !refresh && c.cache != "" && time.Now().Before(c.cacheExpiry) {
		cached := c.cache
		c.mu.Unlock()
		return cached, nil
	}
	c.mu.Unlock()

	report, err := c.collectReport(ctx)
	if err != nil {
		return "", err
	}

	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode system info: %w", err)
	}

	c.mu.Lock()
	c.cache = string(encoded)
	c.cacheExpiry = time.Now().Add(c.cacheTTL)
	cached := c.cache
	c.mu.Unlock()

	return cached, nil
}

func (c *SystemInfoCollector) collectReport(ctx context.Context) (*SystemInfoReport, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	report := &SystemInfoReport{
		CollectedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Hardware: SystemInfoHardware{
			Architecture: runtime.GOARCH,
			LogicalCores: runtime.NumCPU(),
		},
		Runtime: SystemInfoRuntime{
			GoVersion:   runtime.Version(),
			GoOS:        runtime.GOOS,
			GoArch:      runtime.GOARCH,
			LogicalCPUs: runtime.NumCPU(),
			GoMaxProcs:  runtime.GOMAXPROCS(0),
			Goroutines:  runtime.NumGoroutine(),
			Process: SystemInfoProcess{
				PID: os.Getpid(),
			},
		},
		Environment: SystemInfoEnvironment{
			PathSeparator: string(os.PathSeparator),
			TempDir:       os.TempDir(),
		},
		Agent: SystemInfoAgent{
			ID:            c.agent.id,
			Version:       fallback(c.agent.metadata.Version, buildVersion),
			StartTime:     c.agent.startTime.UTC().Format(time.RFC3339Nano),
			UptimeSeconds: uint64(time.Since(c.agent.startTime).Round(time.Second) / time.Second),
			Tags:          cloneStrings(c.agent.metadata.Tags),
		},
	}

	var warnings []string

	if err := c.populateHost(ctx, report, &warnings); err != nil {
		warnings = append(warnings, err.Error())
	}
	if err := c.populateCPU(ctx, report, &warnings); err != nil {
		warnings = append(warnings, err.Error())
	}
	if err := c.populateMemory(ctx, report, &warnings); err != nil {
		warnings = append(warnings, err.Error())
	}
	if err := c.populateStorage(ctx, report, &warnings); err != nil {
		warnings = append(warnings, err.Error())
	}
	if err := c.populateNetwork(ctx, report, &warnings); err != nil {
		warnings = append(warnings, err.Error())
	}
	if err := c.populateProcess(ctx, report, &warnings); err != nil {
		warnings = append(warnings, err.Error())
	}
	c.populateEnvironment(report)

	if len(warnings) > 0 {
		report.Warnings = warnings
	}

	return report, nil
}

func (c *SystemInfoCollector) populateHost(ctx context.Context, report *SystemInfoReport, warnings *[]string) error {
	hostInfo, err := host.InfoWithContext(ctx)
	if err != nil {
		report.Host.Hostname = fallback(report.Host.Hostname, c.agent.metadata.Hostname)
		report.Host.IPAddress = fallback(report.Host.IPAddress, c.agent.metadata.IPAddress)
		return fmt.Errorf("host info: %w", err)
	}

	report.Host.Hostname = fallback(hostInfo.Hostname, c.agent.metadata.Hostname)
	report.Host.HostID = hostInfo.HostID
	report.Host.UptimeSeconds = hostInfo.Uptime
	if hostInfo.BootTime > 0 {
		report.Host.BootTime = time.Unix(int64(hostInfo.BootTime), 0).UTC().Format(time.RFC3339Nano)
	}
	report.Host.IPAddress = fallback(c.agent.metadata.IPAddress, report.Host.IPAddress)

	if report.Host.Domain == "" {
		report.Host.Domain = fallback(os.Getenv("USERDOMAIN"), os.Getenv("DOMAIN"))
	}

	if tzName, offset := time.Now().Zone(); tzName != "" || offset != 0 {
		report.Host.Timezone = formatTimezone(tzName, offset)
	}

	report.OS = SystemInfoOS{
		Platform:       hostInfo.Platform,
		Family:         hostInfo.PlatformFamily,
		Version:        hostInfo.PlatformVersion,
		KernelVersion:  hostInfo.KernelVersion,
		KernelArch:     hostInfo.KernelArch,
		Procs:          hostInfo.Procs,
		Virtualization: hostInfo.VirtualizationSystem,
	}

	report.Hardware.VirtualizationSystem = hostInfo.VirtualizationSystem
	report.Hardware.VirtualizationRole = hostInfo.VirtualizationRole

	return nil
}

func (c *SystemInfoCollector) populateCPU(ctx context.Context, report *SystemInfoReport, warnings *[]string) error {
	physical, err := cpu.CountsWithContext(ctx, false)
	if err == nil {
		report.Hardware.PhysicalCores = physical
	} else {
		*warnings = append(*warnings, fmt.Sprintf("physical cpu count: %v", err))
	}

	logical, err := cpu.CountsWithContext(ctx, true)
	if err == nil {
		report.Hardware.LogicalCores = logical
	} else {
		*warnings = append(*warnings, fmt.Sprintf("logical cpu count: %v", err))
	}

	infos, err := cpu.InfoWithContext(ctx)
	if err != nil {
		return fmt.Errorf("cpu info: %w", err)
	}

	cpus := make([]SystemInfoCPU, 0, len(infos))
	for idx, info := range infos {
		if err := ctx.Err(); err != nil {
			return err
		}
		var cacheSize uint32
		if info.CacheSize > 0 {
			cacheSize = uint32(info.CacheSize)
		}
		cpus = append(cpus, SystemInfoCPU{
			ID:        idx,
			Model:     info.ModelName,
			Vendor:    info.VendorID,
			Family:    info.Family,
			CacheSize: cacheSize,
			Mhz:       info.Mhz,
			Stepping:  info.Stepping,
			Microcode: info.Microcode,
			Cores:     info.Cores,
		})
	}
	report.Hardware.CPUs = cpus
	return nil
}

func (c *SystemInfoCollector) populateMemory(ctx context.Context, report *SystemInfoReport, warnings *[]string) error {
	vmStat, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return fmt.Errorf("virtual memory: %w", err)
	}
	report.Memory.TotalBytes = vmStat.Total
	report.Memory.AvailableBytes = vmStat.Available
	report.Memory.UsedBytes = vmStat.Used
	report.Memory.UsedPercent = round(vmStat.UsedPercent, 2)

	swapStat, err := mem.SwapMemoryWithContext(ctx)
	if err != nil {
		*warnings = append(*warnings, fmt.Sprintf("swap memory: %v", err))
	} else {
		report.Memory.SwapTotalBytes = swapStat.Total
		report.Memory.SwapFreeBytes = swapStat.Free
		report.Memory.SwapUsedBytes = swapStat.Used
		report.Memory.SwapUsedPercent = round(swapStat.UsedPercent, 2)
	}
	return nil
}

func (c *SystemInfoCollector) populateStorage(ctx context.Context, report *SystemInfoReport, warnings *[]string) error {
	partitions, err := disk.PartitionsWithContext(ctx, true)
	if err != nil {
		return fmt.Errorf("disk partitions: %w", err)
	}
	storages := make([]SystemInfoStorage, 0, len(partitions))
	for _, part := range partitions {
		if err := ctx.Err(); err != nil {
			return err
		}
		usage, err := disk.UsageWithContext(ctx, part.Mountpoint)
		if err != nil {
			*warnings = append(*warnings, fmt.Sprintf("disk usage (%s): %v", part.Mountpoint, err))
			continue
		}
		storages = append(storages, SystemInfoStorage{
			Device:      fallback(part.Device, part.Mountpoint),
			Mountpoint:  part.Mountpoint,
			Filesystem:  part.Fstype,
			TotalBytes:  usage.Total,
			UsedBytes:   usage.Used,
			FreeBytes:   usage.Free,
			UsedPercent: round(usage.UsedPercent, 2),
			ReadOnly:    isReadOnly(part.Opts),
		})
	}
	sort.Slice(storages, func(i, j int) bool {
		if storages[i].Mountpoint == storages[j].Mountpoint {
			return storages[i].Device < storages[j].Device
		}
		return storages[i].Mountpoint < storages[j].Mountpoint
	})
	report.Storage = storages
	return nil
}

func (c *SystemInfoCollector) populateNetwork(ctx context.Context, report *SystemInfoReport, warnings *[]string) error {
	interfaces, err := gnet.InterfacesWithContext(ctx)
	if err != nil {
		return fmt.Errorf("network interfaces: %w", err)
	}
	ifaces := make([]SystemInfoNetworkInterface, 0, len(interfaces))
	for _, iface := range interfaces {
		if err := ctx.Err(); err != nil {
			return err
		}
		addresses := make([]SystemInfoNetworkAddress, 0, len(iface.Addrs))
		for _, addr := range iface.Addrs {
			address := strings.TrimSpace(addr.Addr)
			if address == "" {
				continue
			}
			family := ""
			if strings.Contains(address, ":") {
				family = "ipv6"
			} else {
				family = "ipv4"
			}
			addresses = append(addresses, SystemInfoNetworkAddress{
				Address: address,
				Family:  family,
			})
		}
		ifaces = append(ifaces, SystemInfoNetworkInterface{
			Name:      iface.Name,
			MTU:       int(iface.MTU),
			Hardware:  iface.HardwareAddr,
			Flags:     cloneStrings(iface.Flags),
			Addresses: addresses,
		})
	}
	sort.Slice(ifaces, func(i, j int) bool {
		return ifaces[i].Name < ifaces[j].Name
	})
	report.Network = ifaces
	if report.Host.IPAddress == "" {
		report.Host.IPAddress = selectPrimaryIPAddress(ifaces)
	}
	return nil
}

func (c *SystemInfoCollector) populateProcess(ctx context.Context, report *SystemInfoReport, warnings *[]string) error {
	executable, err := os.Executable()
	if err == nil {
		report.Runtime.Process.Executable = executable
	}

	proc, err := process.NewProcessWithContext(ctx, int32(report.Runtime.Process.PID))
	if err != nil {
		return fmt.Errorf("process info: %w", err)
	}

	if ppid, err := proc.PpidWithContext(ctx); err == nil {
		report.Runtime.Process.ParentPID = int(ppid)
	}
	if cmdline, err := proc.CmdlineSliceWithContext(ctx); err == nil {
		report.Runtime.Process.CommandLine = strings.Join(cmdline, " ")
	} else if len(os.Args) > 0 {
		report.Runtime.Process.CommandLine = strings.Join(os.Args, " ")
	}
	if cwd, err := proc.CwdWithContext(ctx); err == nil {
		report.Runtime.Process.WorkingDirectory = cwd
	} else if cwd, err := os.Getwd(); err == nil {
		report.Runtime.Process.WorkingDirectory = cwd
	}
	if createTime, err := proc.CreateTimeWithContext(ctx); err == nil && createTime > 0 {
		created := time.UnixMilli(createTime).UTC()
		report.Runtime.Process.CreateTime = created.Format(time.RFC3339Nano)
		if now := time.Now(); now.After(created) {
			report.Runtime.Process.UptimeSeconds = now.Sub(created).Seconds()
		}
	}
	if cpuPercent, err := proc.CPUPercentWithContext(ctx); err == nil {
		report.Runtime.Process.CPUPercent = round(cpuPercent, 2)
	}
	if memInfo, err := proc.MemoryInfoWithContext(ctx); err == nil {
		report.Runtime.Process.MemoryRSSBytes = memInfo.RSS
		report.Runtime.Process.MemoryVMSBytes = memInfo.VMS
	}
	if threads, err := proc.NumThreadsWithContext(ctx); err == nil {
		report.Runtime.Process.NumThreads = threads
	}
	return nil
}

func (c *SystemInfoCollector) populateEnvironment(report *SystemInfoReport) {
	currentUser, err := user.Current()
	if err == nil {
		report.Environment.Username = currentUser.Username
		report.Environment.HomeDir = currentUser.HomeDir
	} else {
		if report.Environment.Username == "" {
			report.Environment.Username = fallback(os.Getenv("USER"), os.Getenv("USERNAME"))
		}
		if report.Environment.HomeDir == "" {
			if home, err := os.UserHomeDir(); err == nil {
				report.Environment.HomeDir = home
			}
		}
	}

	report.Environment.Shell = detectShell()
	report.Environment.Lang = detectLocale()
	report.Environment.EnvironmentCount = len(os.Environ())
	report.Environment.PathEntries = parsePathEntries(os.Getenv("PATH"))
	if report.Environment.HomeDir == "" {
		if home, err := os.UserHomeDir(); err == nil {
			report.Environment.HomeDir = home
		}
	}
	if report.Environment.Username == "" {
		report.Environment.Username = c.agent.metadata.Username
	}
}

func detectShell() string {
	if shell := strings.TrimSpace(os.Getenv("SHELL")); shell != "" {
		return shell
	}
	if shell := strings.TrimSpace(os.Getenv("COMSPEC")); shell != "" {
		return shell
	}
	if runtime.GOOS == "windows" {
		systemRoot := os.Getenv("SystemRoot")
		if systemRoot == "" {
			systemRoot = os.Getenv("WINDIR")
		}
		if systemRoot != "" {
			candidate := filepath.Join(systemRoot, "System32", "cmd.exe")
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
		return "cmd.exe"
	}
	return "/bin/sh"
}

func detectLocale() string {
	keys := []string{"LC_ALL", "LC_MESSAGES", "LANG", "LANGUAGE"}
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func parsePathEntries(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parts := strings.Split(value, string(os.PathListSeparator))
	entries := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			entries = append(entries, trimmed)
		}
	}
	if len(entries) == 0 {
		return nil
	}
	return entries
}

func selectPrimaryIPAddress(ifaces []SystemInfoNetworkInterface) string {
	for _, iface := range ifaces {
		for _, flag := range iface.Flags {
			if strings.EqualFold(flag, "loopback") {
				goto nextInterface
			}
		}
		for _, addr := range iface.Addresses {
			if strings.HasPrefix(addr.Address, "127.") || strings.HasPrefix(addr.Address, "::1") {
				continue
			}
			if addr.Address != "" {
				return addr.Address
			}
		}
	nextInterface:
	}
	return ""
}

func isReadOnly(opts []string) bool {
	if len(opts) == 0 {
		return false
	}
	for _, opt := range opts {
		if strings.TrimSpace(strings.ToLower(opt)) == "ro" {
			return true
		}
	}
	return false
}

func round(value float64, precision int) float64 {
	if precision < 0 {
		return value
	}
	factor := math.Pow10(precision)
	return math.Round(value*factor) / factor
}

func formatTimezone(name string, offsetSeconds int) string {
	if name == "" && offsetSeconds == 0 {
		return ""
	}
	sign := "+"
	if offsetSeconds < 0 {
		sign = "-"
		offsetSeconds = -offsetSeconds
	}
	hours := offsetSeconds / 3600
	minutes := (offsetSeconds % 3600) / 60
	if name == "" {
		return fmt.Sprintf("UTC%s%02d:%02d", sign, hours, minutes)
	}
	return fmt.Sprintf("%s (UTC%s%02d:%02d)", name, sign, hours, minutes)
}

func cloneStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, len(values))
	copy(out, values)
	return out
}

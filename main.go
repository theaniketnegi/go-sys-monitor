package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
)

type tickMsg time.Time

var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240"))

var boldPinkStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF7CCB")).Bold(true)

type model struct {
	sys         SystemMetrics
	cpuProgress []progress.Model
	memProgress progress.Model
	diskTable   table.Model
}

type SystemMetrics struct {
	cpu    CPUMetrics
	memory MemoryMetrics
	disk   DiskMetrics
}

type CPUMetrics struct {
	modelName    string
	frequency    float64
	totalCPU     int
	logicalCPU   int
	percentUsage []float64
}

type MemoryMetrics struct {
	memoryUsed  uint64
	memoryTotal uint64
}

type DiskMetrics struct {
	mountedPartitions []MountedPartitionMetrics
}

type MountedPartitionMetrics struct {
	devName    string
	mountPoint string
	fsType     string
	freeDisk   uint64
	usedDisk   uint64
	totalDisk  uint64
}

const (
	TITLE = `
                      _                                        (_)   _                 
  ___  _   _   ___  _| |_  _____  ____     ____    ___   ____   _  _| |_   ___    ____ 
 /___)| | | | /___)(_   _)| ___ ||    \   |    \  / _ \ |  _ \ | |(_   _) / _ \  / ___)
|___ || |_| ||___ |  | |_ | ____|| | | |  | | | || |_| || | | || |  | |_ | |_| || |    
(___/  \__  |(___/    \__)|_____)|_|_|_|  |_|_|_| \___/ |_| |_||_|   \__) \___/ |_|    
      (____/                                                                           

	`
	CPU_TITLE  = "CPU Metrics"
	MEM_TITLE  = "Memory Metrics"
	DISK_TITLE = "Disk metrics"
)

func GetCPUMetrics() (CPUMetrics, error) {
	stat, err := cpu.Info()
	if err != nil {
		return CPUMetrics{}, err
	}

	total, err := cpu.Counts(true)
	if err != nil {
		return CPUMetrics{}, err
	}

	logical, err := cpu.Counts(false)
	if err != nil {
		return CPUMetrics{}, err
	}

	v, err := cpu.Percent(time.Second, true)
	if err != nil {
		return CPUMetrics{}, err
	}

	return CPUMetrics{modelName: stat[0].ModelName, frequency: stat[0].Mhz, totalCPU: total, logicalCPU: logical, percentUsage: v}, nil
}

func GetMemMetrics() (MemoryMetrics, error) {
	v, err := mem.VirtualMemory()
	if err != nil {
		return MemoryMetrics{}, err
	}
	return MemoryMetrics{memoryUsed: v.Used, memoryTotal: v.Total}, nil
}

func GetDiskMetrics() (DiskMetrics, error) {
	partitions, err := disk.Partitions(false)

	if err != nil {
		return DiskMetrics{}, err
	}

	var partitionMetrics []MountedPartitionMetrics
	for _, partition := range partitions {
		stat, err := disk.Usage(partition.Mountpoint)
		if err != nil {
			continue
		}
		partitionMetrics = append(partitionMetrics, MountedPartitionMetrics{devName: partition.Device, mountPoint: partition.Mountpoint, fsType: partition.Fstype, freeDisk: stat.Free, usedDisk: stat.Used, totalDisk: stat.Total})
	}

	return DiskMetrics{partitionMetrics}, nil
}

func doTick() tea.Cmd {
	return tea.Tick(50*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func NewProgress() progress.Model {
	return progress.New(progress.WithScaledGradient("#FF7CCB", "#FDFF8C"))
}

func getDiskTableColumns() []table.Column {
	return []table.Column{
		{Title: "Device", Width: 20},
		{Title: "Mount", Width: 40},
		{Title: "FS", Width: 10},
		{Title: "Used", Width: 10},
		{Title: "Total", Width: 10},
		{Title: "Free", Width: 10},
	}
}

func convertSize(size uint64) string {
	var conv uint64
	var res string

	conv = size / (1024 * 1024)
	if conv >= 1024 {
		conv /= 1024
		res = strconv.FormatUint(conv, 10) + "G"
	} else {
		res = strconv.FormatUint(conv, 10) + "M"
	}

	return res
}

func getDiskTableRows(diskMetrics DiskMetrics) []table.Row {
	var diskTableRow []table.Row

	for _, disk := range diskMetrics.mountedPartitions {
		diskTableRow = append(diskTableRow, table.Row{
			disk.devName, disk.mountPoint, disk.fsType, convertSize(disk.usedDisk), convertSize(disk.totalDisk), convertSize(disk.freeDisk),
		})
	}

	return diskTableRow
}

func initialModel() model {
	initCpu, err := GetCPUMetrics()
	if err != nil {
		log.Fatal("There was some error getting CPU metrics: ", err)
	}

	initMem, err := GetMemMetrics()
	if err != nil {
		log.Fatal("There was some error getting memory metrics: ", err)
	}

	cpuProgress := make([]progress.Model, len(initCpu.percentUsage))

	for i := 0; i < len(cpuProgress); i++ {
		cpuProgress[i] = NewProgress()
	}

	memProgress := NewProgress()

	initDisk, err := GetDiskMetrics()
	if err != nil {
		log.Fatal("There was some error getting disk metrics: ", err)
	}

	t := table.New(
		table.WithColumns(getDiskTableColumns()),
		table.WithRows(getDiskTableRows(initDisk)),
		table.WithHeight(len(initDisk.mountedPartitions)+1),
		table.WithStyles(table.Styles{
			Header: lipgloss.NewStyle().Bold(true),
		}),
	)

	return model{
		sys:         SystemMetrics{cpu: initCpu, memory: initMem, disk: initDisk},
		cpuProgress: cpuProgress,
		memProgress: memProgress,
		diskTable:   t,
	}
}

func (m model) Init() tea.Cmd {
	return doTick()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// Is it a key press?
	case tea.KeyMsg:

		// Cool, what was the actual key pressed?
		switch msg.String() {

		// These keys should exit the program.
		case "ctrl+c", "q":
			return m, tea.Quit
		}

	case tickMsg:
		cpu, err := GetCPUMetrics()
		if err != nil {
			log.Fatal("There was some error getting CPU metrics: ", err)
		}

		mem, err := GetMemMetrics()
		if err != nil {
			log.Fatal("There was some error getting memory metrics: ", err)
		}

		disk, err := GetDiskMetrics()
		if err != nil {
			log.Fatal("There was some error getting disk metrics: ", err)
		}

		m.diskTable.SetRows(getDiskTableRows(disk))
		m.diskTable.SetHeight(len(disk.mountedPartitions) + 1)
		m.sys.cpu = cpu
		m.sys.memory = mem
		m.sys.disk = disk
		return m, doTick()
	}

	// Return the updated model to the Bubble Tea runtime for processing.
	// Note that we're not returning a command.
	return m, nil
}

func (m model) View() string {
	var s strings.Builder
	s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FDFF8C")).Render(TITLE))
	s.WriteString("\n\n")
	s.WriteString(boldPinkStyle.Render(CPU_TITLE) + "\n")
	s.WriteString(boldPinkStyle.Render(strings.Repeat("#", len(CPU_TITLE))) + "\n")
	s.WriteString(fmt.Sprintf("Model Name: %s\n", m.sys.cpu.modelName))
	s.WriteString(fmt.Sprintf("Frequency: %.2fMHz\n", m.sys.cpu.frequency))
	s.WriteString(fmt.Sprintf("Total CPU: %d (%d Logical)\n", m.sys.cpu.totalCPU, m.sys.cpu.logicalCPU))

	for i, per := range m.sys.cpu.percentUsage {
		s.WriteString(fmt.Sprintf("\nCPU %d: ", i+1) + m.cpuProgress[i].ViewAs(per/100))
	}

	s.WriteString("\n\n")
	s.WriteString(boldPinkStyle.Render(MEM_TITLE) + "\n")
	s.WriteString(boldPinkStyle.Render(strings.Repeat("#", len(MEM_TITLE))) + "\n")
	s.WriteString(fmt.Sprintf("Used %d bytes (of %d bytes)\n\n", m.sys.memory.memoryUsed, m.sys.memory.memoryTotal))
	s.WriteString(m.memProgress.ViewAs(float64(m.sys.memory.memoryUsed) / float64(m.sys.memory.memoryTotal)))

	s.WriteString("\n\n")
	s.WriteString(boldPinkStyle.Render(DISK_TITLE) + "\n")
	s.WriteString(boldPinkStyle.Render(strings.Repeat("#", len(DISK_TITLE))) + "\n")

	s.WriteString(baseStyle.Render(m.diskTable.View()))
	return s.String()
}

func main() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}

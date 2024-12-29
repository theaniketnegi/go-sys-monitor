package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
)

type tickMsg time.Time

type model struct {
	sys         SystemMetrics
	cpuProgress []progress.Model
	memProgress progress.Model
}

type SystemMetrics struct {
	cpu    CPUMetrics
	memory MemoryMetrics
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

	GetDiskMetrics()
	return model{
		sys:         SystemMetrics{cpu: initCpu, memory: initMem},
		cpuProgress: cpuProgress,
		memProgress: memProgress,
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
		m.sys.cpu = cpu
		m.sys.memory = mem
		return m, doTick()
	}

	// Return the updated model to the Bubble Tea runtime for processing.
	// Note that we're not returning a command.
	return m, nil
}

func (m model) View() string {

	s := "System Monitor\n\n"
	s += CPU_TITLE + "\n"
	s += strings.Repeat("#", len(CPU_TITLE))
	s += "\n\n"
	s += fmt.Sprintf("Model Name: %s\n", m.sys.cpu.modelName)
	s += fmt.Sprintf("Frequency: %.2fMHz\n", m.sys.cpu.frequency)
	s += fmt.Sprintf("Total CPU: %d (%d Logical)\n", m.sys.cpu.totalCPU, m.sys.cpu.logicalCPU)

	for i, per := range m.sys.cpu.percentUsage {
		s += fmt.Sprintf("\nCPU %d: ", i+1) + m.cpuProgress[i].ViewAs(per/100)
	}

	s += "\n\n"
	s += MEM_TITLE + "\n"
	s += strings.Repeat("#", len(MEM_TITLE))
	s += "\n\n"
	s += fmt.Sprintf("Used %d bytes (of %d bytes)\n\n", m.sys.memory.memoryUsed, m.sys.memory.memoryTotal)
	s += m.memProgress.ViewAs(float64(m.sys.memory.memoryUsed) / float64(m.sys.memory.memoryTotal))
	return s
}

func main() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}

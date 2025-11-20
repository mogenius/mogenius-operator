package cpumonitor

import (
	"fmt"
	"log/slog"
	"mogenius-operator/src/assert"
	"mogenius-operator/src/config"
	"mogenius-operator/src/containerenumerator"
	"mogenius-operator/src/k8sclient"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/tklauser/go-sysconf"
)

type CpuMonitor interface {
	CpuUsageGlobal() CpuMetrics
	CpuUsageProcesses() []PodCpuStats
}

type cpuMonitor struct {
	SC_CLK_TCK uint64 // loaded from sysconf: the kernels amount of ticks per second for scheduling tasks

	logger              *slog.Logger
	config              config.ConfigModule
	clientProvider      k8sclient.K8sClientProvider
	containerEnumerator containerenumerator.ContainerEnumerator
	procPath            string

	running atomic.Bool

	cpuUsageGlobalTx chan struct{}
	cpuUsageGlobalRx chan CpuMetrics

	cpuUsageProcessesTx chan struct{}
	cpuUsageProcessesRx chan map[containerenumerator.ContainerId][]ProcPidStat
}

func NewCpuMonitor(
	logger *slog.Logger,
	config config.ConfigModule,
	clientProvider k8sclient.K8sClientProvider,
	containerEnumerator containerenumerator.ContainerEnumerator,
) CpuMonitor {
	self := &cpuMonitor{}

	self.logger = logger
	self.config = config
	self.clientProvider = clientProvider
	self.containerEnumerator = containerEnumerator
	self.procPath = config.Get("MO_HOST_PROC_PATH")
	self.running = atomic.Bool{}

	clktck, err := sysconf.Sysconf(sysconf.SC_CLK_TCK)
	assert.Assert(err == nil, err)
	self.SC_CLK_TCK = uint64(clktck)

	self.cpuUsageGlobalTx = make(chan struct{})
	self.cpuUsageGlobalRx = make(chan CpuMetrics)
	self.cpuUsageProcessesTx = make(chan struct{})
	self.cpuUsageProcessesRx = make(chan map[containerenumerator.ContainerId][]ProcPidStat)

	return self
}

func (self *cpuMonitor) CpuUsageGlobal() CpuMetrics {
	self.startCollector()

	self.cpuUsageGlobalTx <- struct{}{}
	metrics := <-self.cpuUsageGlobalRx

	return metrics
}

func (self *cpuMonitor) startCollector() {
	wasRunning := self.running.Swap(true)
	if wasRunning {
		return
	}
	if runtime.GOOS != "linux" {
		return
	}
	go func() {
		statPath := filepath.Join(self.procPath, "stat")

		globalMetricsUpdater := time.NewTicker(1 * time.Second)
		defer globalMetricsUpdater.Stop()

		processMetricsUpdater := time.NewTicker(1 * time.Second)
		defer processMetricsUpdater.Stop()

		lastProcStatCpu := ProcStatCpu{}
		lastProcStatCpu, metrics := self.updateMetrics(statPath, lastProcStatCpu)
		processMetrics := self.collectProcessMetrics()

		for {
			select {
			case <-globalMetricsUpdater.C:
				lastProcStatCpu, metrics = self.updateMetrics(statPath, lastProcStatCpu)
			case <-processMetricsUpdater.C:
				processMetrics = self.collectProcessMetrics()
			case <-self.cpuUsageGlobalTx:
				self.cpuUsageGlobalRx <- metrics
			case <-self.cpuUsageProcessesTx:
				self.cpuUsageProcessesRx <- processMetrics
			}
		}
	}()
}

type ProcStatCpu struct {
	// user   (1) Time spent in user mode.
	User uint64 `json:"user"`

	// nice   (2) Time spent in user mode with low priority (nice).
	Nice uint64 `json:"nice"`

	// system (3) Time spent in system mode.
	System uint64 `json:"system"`

	// idle   (4) Time spent in the idle task.  This value should be USER_HZ times the second entry in the /proc/uptime pseudo-file.
	Idle uint64 `json:"idle"`

	// iowait (since Linux 2.5.41)
	//        (5) Time waiting for I/O to complete.  This value is not reliable, for the following reasons:
	//        •  The CPU will not wait for I/O to complete; iowait is the time that a task is waiting for I/O to complete.  When a CPU goes into idle state for outstanding task I/O, another task will be scheduled on this CPU.
	//        •  On a multi-core CPU, the task waiting for I/O to complete is not running on any CPU, so the iowait of each CPU is difficult to calculate.
	//        •  The value in this field may decrease in certain conditions.
	Iowait uint64 `json:"iowait"`

	// irq (since Linux 2.6.0)
	//        (6) Time servicing interrupts.
	Irq uint64 `json:"irq"`

	// softirq (since Linux 2.6.0)
	//        (7) Time servicing softirqs.
	Softirq uint64 `json:"softirq"`

	// steal (since Linux 2.6.11)
	//        (8) Stolen time, which is the time spent in other operating systems when running in a virtualized environment
	Steal uint64 `json:"steal"`

	// guest (since Linux 2.6.24)
	//        (9) Time spent running a virtual CPU for guest operating systems under the control of the Linux kernel.
	Guest uint64 `json:"guest"`

	// guest_nice (since Linux 2.6.33)
	//        (10) Time spent running a niced guest (virtual CPU for guest operating systems under the control of the Linux kernel).
	GuestNice uint64 `json:"guest_nice"`
}

func (self *cpuMonitor) updateMetrics(statPath string, lastProcStatCpu ProcStatCpu) (ProcStatCpu, CpuMetrics) {
	procStatCpu, err := self.readGlobalCpuMetrics(statPath)
	if err != nil {
		self.logger.Warn("failed to read initial cpu metrics", "sourcepath", statPath, "error", err)
	}
	metrics := self.calculateGlobalMetrics(lastProcStatCpu, procStatCpu)

	return procStatCpu, metrics
}

func (self *cpuMonitor) readGlobalCpuMetrics(statPath string) (ProcStatCpu, error) {
	data := ProcStatCpu{}

	asUint64 := func(val string) uint64 {
		intval, err := strconv.ParseUint(val, 10, 64)
		assert.Assert(err == nil, "val is expected to be an uint64", val, err)
		return intval
	}

	fileData, err := os.ReadFile(statPath)
	if err != nil {
		return ProcStatCpu{}, fmt.Errorf("failed to read cpu stats: %s", err.Error())
	}

	lines := strings.Split(string(fileData), "\n")
	cpuLine := lines[0]
	assert.Assert(cpuLine != "")

	fields := strings.Fields(cpuLine)
	assert.Assert(fields[0] == "cpu", "expected to find key 'cpu' at first place", fields[0])
	data.User = asUint64(fields[1])
	data.Nice = asUint64(fields[2])
	data.System = asUint64(fields[3])
	data.Idle = asUint64(fields[4])
	data.Iowait = asUint64(fields[5])
	data.Irq = asUint64(fields[6])
	data.Softirq = asUint64(fields[7])
	data.Steal = asUint64(fields[8])
	data.Guest = asUint64(fields[9])
	data.GuestNice = asUint64(fields[10])

	return data, nil
}

func (self *cpuMonitor) calculateGlobalMetrics(lastProcStatCpu ProcStatCpu, procStatCpu ProcStatCpu) CpuMetrics {
	// Calculate deltas
	userDelta := float64(procStatCpu.User - lastProcStatCpu.User)
	niceDelta := float64(procStatCpu.Nice - lastProcStatCpu.Nice)
	systemDelta := float64(procStatCpu.System - lastProcStatCpu.System)
	idleDelta := float64(procStatCpu.Idle - lastProcStatCpu.Idle)
	iowaitDelta := float64(procStatCpu.Iowait - lastProcStatCpu.Iowait)
	irqDelta := float64(procStatCpu.Irq - lastProcStatCpu.Irq)
	softirqDelta := float64(procStatCpu.Softirq - lastProcStatCpu.Softirq)
	stealDelta := float64(procStatCpu.Steal - lastProcStatCpu.Steal)

	// Total time delta
	totalDelta := userDelta + niceDelta + systemDelta + idleDelta +
		iowaitDelta + irqDelta + softirqDelta + stealDelta

	// Calculate percentages
	data := CpuMetrics{}
	data.User = ((userDelta + niceDelta) / totalDelta) * 100                   // user + nice
	data.System = ((systemDelta + irqDelta + softirqDelta) / totalDelta) * 100 // system + irq + softirq
	data.Idle = ((idleDelta + iowaitDelta) / totalDelta) * 100                 // idle + iowait

	return data
}

type CpuMetrics struct {
	User   float64 `json:"user"`
	System float64 `json:"system"`
	Idle   float64 `json:"idle"`
}

func (self *cpuMonitor) collectProcessMetrics() map[containerenumerator.ContainerId][]ProcPidStat {
	data := map[containerenumerator.ContainerId][]ProcPidStat{}
	containers := self.containerEnumerator.GetProcessesWithContainerIds()

	for containerId, pids := range containers {
		infos := []ProcPidStat{}
		for _, pid := range pids {
			info, err := getCpuUsageInfo(self.procPath, strconv.FormatUint(pid, 10))
			if err != nil {
				continue
			}
			infos = append(infos, info)
		}
		data[containerId] = infos
	}

	return data
}

func (self *cpuMonitor) CpuUsageProcesses() []PodCpuStats {
	self.startCollector()

	self.cpuUsageProcessesTx <- struct{}{}
	data := <-self.cpuUsageProcessesRx

	pods := self.containerEnumerator.GetPodsWithContainerIds()

	stats := []PodCpuStats{}
	uptimepath := filepath.Join(self.procPath, "uptime")
	uptimedata, err := os.ReadFile(filepath.Join(self.procPath, "uptime"))
	if err != nil {
		self.logger.Error("failed to read uptime", "uptimepath", uptimepath, "error", err)
		return stats
	}
	uptimefields := strings.Fields(string(uptimedata))
	uptime, err := strconv.ParseFloat(uptimefields[0], 64)
	if err != nil {
		self.logger.Error("failed to parse uptime as float", "uptimepath", uptimepath, "error", err)
		return stats
	}

	for _, pod := range pods {
		for containerId := range pod.Containers {
			procPidStatm, ok := data[containerId]
			if !ok {
				continue
			}
			podCpuStats := self.procPidStatToCpuUsage(pod, procPidStatm, uptime)
			stats = append(stats, podCpuStats)
		}
	}

	return stats
}

func (self *cpuMonitor) procPidStatToCpuUsage(pod containerenumerator.PodInfo, stats []ProcPidStat, uptime float64) PodCpuStats {
	data := PodCpuStats{}
	data.Name = pod.Name
	data.Namespace = pod.Namespace
	data.StartTime = pod.StartTime
	data.Pids = make([]CpuUsagePodPid, 0, len(stats))

	for _, stat := range stats {
		pidData := CpuUsagePodPid{}
		pidData.Pid = uint64(stat.Pid)
		pidData.BinaryName = stat.Comm[1 : len(stat.Comm)-1]
		pidData.TimeSinceStart = uint64(uptime*1000) - stat.Starttime
		pidData.UserTime = stat.Utime * (1000 / self.SC_CLK_TCK)
		pidData.UserTimeChildren = stat.Cutime * (1000 / self.SC_CLK_TCK)
		pidData.SysTime = stat.Stime * (1000 / self.SC_CLK_TCK)
		pidData.SysTimeChildren = stat.Cstime * (1000 / self.SC_CLK_TCK)
		data.Pids = append(data.Pids, pidData)
	}

	return data
}

type PodCpuStats struct {
	Name      string           `json:"name"`
	Namespace string           `json:"namespace"`
	StartTime string           `json:"start_time"`
	Pids      []CpuUsagePodPid `json:"pids"`
}

type CpuUsagePodPid struct {
	Pid              uint64 `json:"pid"`
	BinaryName       string `json:"binary_name"`
	TimeSinceStart   uint64 `json:"time_since_start"`
	UserTime         uint64 `json:"user_time"`
	UserTimeChildren uint64 `json:"user_time_children,omitempty"`
	SysTime          uint64 `json:"sys_time"`
	SysTimeChildren  uint64 `json:"sys_time_children,omitempty"`
}

type ProcPidStat struct {
	// (1) pid  %d
	//        The process ID.
	Pid int64 `json:"pid"`

	// (2) comm  %s
	//        The filename of the executable, in parentheses.  Strings longer than TASK_COMM_LEN (16) characters (including the terminating null byte) are silently truncated.  This is visible whether or not the executable is swapped out.
	Comm string `json:"comm"`

	// (3) state  %c
	//        One of the following characters, indicating process state:
	//
	//        R      Running
	//
	//        S      Sleeping in an interruptible wait
	//
	//        D      Waiting in uninterruptible disk sleep
	//
	//        Z      Zombie
	//
	//        T      Stopped (on a signal) or (before Linux 2.6.33) trace stopped
	//
	//        t      Tracing stop (Linux 2.6.33 onward)
	//
	//        W      Paging (only before Linux 2.6.0)
	//
	//        X      Dead (from Linux 2.6.0 onward)
	//
	//        x      Dead (Linux 2.6.33 to 3.13 only)
	//
	//        K      Wakekill (Linux 2.6.33 to 3.13 only)
	//
	//        W      Waking (Linux 2.6.33 to 3.13 only)
	//
	//        P      Parked (Linux 3.9 to 3.13 only)
	//
	//        I      Idle (Linux 4.14 onward)
	// State string `json:"state"`

	// (4) ppid  %d
	//        The PID of the parent of this process.
	// Ppid int64 `json:"ppid"`

	// (5) pgrp  %d
	//        The process group ID of the process.
	// Pgrp int64 `json:"pgrp"`

	// (6) session  %d
	//        The session ID of the process.
	// Session int64 `json:"session"`

	// (7) tty_nr  %d
	//        The controlling terminal of the process.  (The minor device number is contained in the combination of bits 31 to 20 and 7 to 0; the major device number is in bits 15 to 8.)
	// TtyNr int64 `json:"tty_nr"`

	// (8) tpgid  %d
	//        The ID of the foreground process group of the controlling terminal of the process.
	// Tpgid int64 `json:"tpgid"`

	// (9) flags  %u
	//        The kernel flags word of the process.  For bit meanings, see the PF_* defines in the Linux kernel source file include/linux/sched.h.  Details depend on the kernel version.
	//
	//        The format for this field was %lu before Linux 2.6.
	// Flags uint64 `json:"flags"`

	// (10) minflt  %lu
	//        The number of minor faults the process has made which have not required loading a memory page from disk.
	// Minflt uint64 `json:"minflt"`

	// (11) cminflt  %lu
	//        The number of minor faults that the process's waited-for children have made.
	// Cminflt uint64 `json:"cminflt"`

	// (12) majflt  %lu
	//        The number of major faults the process has made which have required loading a memory page from disk.
	// Majflt uint64 `json:"majflt"`

	// (13) cmajflt  %lu
	//        The number of major faults that the process's waited-for children have made.
	// Cmajflt uint64 `json:"cmajflt"`

	// (14) utime  %lu
	//        Amount of time that this process has been scheduled in user mode, measured in clock ticks (divide by sysconf(_SC_CLK_TCK)).  This includes guest time, guest_time (time spent running a virtual CPU, see below), so that applications that are not aware  of
	//        the guest time field do not lose that time from their calculations.
	Utime uint64 `json:"utime"`

	// (15) stime  %lu
	//        Amount of time that this process has been scheduled in kernel mode, measured in clock ticks (divide by sysconf(_SC_CLK_TCK)).
	Stime uint64 `json:"stime"`

	// (16) cutime  %ld
	//        Amount  of  time that this process's waited-for children have been scheduled in user mode, measured in clock ticks (divide by sysconf(_SC_CLK_TCK)).  (See also times(2).)  This includes guest time, cguest_time (time spent running a virtual CPU, see be‐
	//        low).
	Cutime uint64 `json:"cutime"`

	// (17) cstime  %ld
	//        Amount of time that this process's waited-for children have been scheduled in kernel mode, measured in clock ticks (divide by sysconf(_SC_CLK_TCK)).
	Cstime uint64 `json:"cstime"`

	// (18) priority  %ld
	//        (Explanation for Linux 2.6) For processes running a real-time scheduling policy (policy below; see sched_setscheduler(2)), this is the negated scheduling priority, minus one; that is, a number in the range -2 to -100, corresponding to real-time priori‐
	//        ties 1 to 99.  For processes running under a non-real-time scheduling policy, this is the raw nice value (setpriority(2)) as represented in the kernel.  The kernel stores nice values as numbers in the range 0 (high) to 39 (low),  corresponding  to  the
	//        user-visible nice range of -20 to 19.
	//
	//        Before Linux 2.6, this was a scaled value based on the scheduler weighting given to this process.
	// Priority int64 `json:"priority"`

	// (19) nice  %ld
	//        The nice value (see setpriority(2)), a value in the range 19 (low priority) to -20 (high priority).
	// Nice int64 `json:"nice"`

	// (20) num_threads  %ld
	//        Number of threads in this process (since Linux 2.6).  Before Linux 2.6, this field was hard coded to 0 as a placeholder for an earlier removed field.
	// NumThreads int64 `json:"num_threads"`

	// (21) itrealvalue  %ld
	//        The time in jiffies before the next SIGALRM is sent to the process due to an interval timer.  Since Linux 2.6.17, this field is no longer maintained, and is hard coded as 0.
	// Itrealvalue int64 `json:"itrealvalue"`

	// (22) starttime  %llu
	//        The time the process started after system boot.  Before Linux 2.6, this value was expressed in jiffies.  Since Linux 2.6, the value is expressed in clock ticks (divide by sysconf(_SC_CLK_TCK)).
	//
	//        The format for this field was %lu before Linux 2.6.
	Starttime uint64 `json:"starttime"`

	// (23) vsize  %lu
	//        Virtual memory size in bytes.
	// Vsize uint64 `json:"vsize"`

	// (24) rss  %ld
	//        Resident  Set Size: number of pages the process has in real memory.  This is just the pages which count toward text, data, or stack space.  This does not include pages which have not been demand-loaded in, or which are swapped out.  This value is inac‐
	//        curate; see /proc/pid/statm below.
	// Rss int64 `json:"rss"`

	// (25) rsslim  %lu
	//        Current soft limit in bytes on the rss of the process; see the description of RLIMIT_RSS in getrlimit(2).
	// Rsslim uint64 `json:"rsslim"`

	// (26) startcode  %lu  [PT]
	//        The address above which program text can run.
	// Startcode uint64 `json:"startcode"`

	// (27) endcode  %lu  [PT]
	//        The address below which program text can run.
	// Endcode uint64 `json:"endcode"`

	// (28) startstack  %lu  [PT]
	//        The address of the start (i.e., bottom) of the stack.
	// Startstack uint64 `json:"startstack"`

	// (29) kstkesp  %lu  [PT]
	//        The current value of ESP (stack pointer), as found in the kernel stack page for the process.
	// Kstkesp uint64 `json:"kstkesp"`

	// (30) kstkeip  %lu  [PT]
	//        The current EIP (instruction pointer).
	// Kstkeip uint64 `json:"kstkeip"`

	// (31) signal  %lu
	//        The bitmap of pending signals, displayed as a decimal number.  Obsolete, because it does not provide information on real-time signals; use /proc/pid/status instead.
	// Signal uint64 `json:"signal"`

	// (32) blocked  %lu
	//        The bitmap of blocked signals, displayed as a decimal number.  Obsolete, because it does not provide information on real-time signals; use /proc/pid/status instead.
	// Blocked uint64 `json:"blocked"`

	// (33) sigignore  %lu
	//        The bitmap of ignored signals, displayed as a decimal number.  Obsolete, because it does not provide information on real-time signals; use /proc/pid/status instead.
	// Sigignore uint64 `json:"sigignore"`

	// (34) sigcatch  %lu
	//        The bitmap of caught signals, displayed as a decimal number.  Obsolete, because it does not provide information on real-time signals; use /proc/pid/status instead.
	// Sigcatch uint64 `json:"sigcatch"`

	// (35) wchan  %lu  [PT]
	//        This is the "channel" in which the process is waiting.  It is the address of a location in the kernel where the process is sleeping.  The corresponding symbolic name can be found in /proc/pid/wchan.
	// Wchan uint64 `json:"wchan"`

	// (36) nswap  %lu
	//        Number of pages swapped (not maintained).
	// Nswap uint64 `json:"nswap"`

	// (37) cnswap  %lu
	//        Cumulative nswap for child processes (not maintained).
	// Cnswap uint64 `json:"cnswap"`

	// (38) exit_signal  %d  (since Linux 2.1.22)
	//        Signal to be sent to parent when we die.
	// ExitSignal uint64 `json:"exit_signal"`

	// (39) processor  %d  (since Linux 2.2.8)
	//        CPU number last executed on.
	// Processor int64 `json:"processor"`

	// (40) rt_priority  %u  (since Linux 2.5.19)
	//        Real-time scheduling priority, a number in the range 1 to 99 for processes scheduled under a real-time policy, or 0, for non-real-time processes (see sched_setscheduler(2)).
	// RtPriority uint64 `json:"rt_priority"`

	// (41) policy  %u  (since Linux 2.5.19)
	//        Scheduling policy (see sched_setscheduler(2)).  Decode using the SCHED_* constants in linux/sched.h.
	//
	//        The format for this field was %lu before Linux 2.6.22.
	// Policy uint64 `json:"policy"`

	// (42) delayacct_blkio_ticks  %llu  (since Linux 2.6.18)
	//        Aggregated block I/O delays, measured in clock ticks (centiseconds).
	// DelayacctBlkioTicks uint64 `json:"delayacct_blkio_ticks"`

	// (43) guest_time  %lu  (since Linux 2.6.24)
	//        Guest time of the process (time spent running a virtual CPU for a guest operating system), measured in clock ticks (divide by sysconf(_SC_CLK_TCK)).
	// GuestTime uint64 `json:"guest_time"`

	// (44) cguest_time  %ld  (since Linux 2.6.24)
	//        Guest time of the process's children, measured in clock ticks (divide by sysconf(_SC_CLK_TCK)).
	// CguestTime int64 `json:"cguest_time"`

	// (45) start_data  %lu  (since Linux 3.3)  [PT]
	//        Address above which program initialized and uninitialized (BSS) data are placed.
	// StartData uint64 `json:"start_data"`

	// (46) end_data  %lu  (since Linux 3.3)  [PT]
	//        Address below which program initialized and uninitialized (BSS) data are placed.
	// EndData uint64 `json:"end_data"`

	// (47) start_brk  %lu  (since Linux 3.3)  [PT]
	//        Address above which program heap can be expanded with brk(2).
	// StartBrk uint64 `json:"start_brk"`

	// (48) arg_start  %lu  (since Linux 3.5)  [PT]
	//        Address above which program command-line arguments (argv) are placed.
	// ArgStart uint64 `json:"arg_start"`

	// (49) arg_end  %lu  (since Linux 3.5)  [PT]
	//        Address below program command-line arguments (argv) are placed.
	// ArgEnd uint64 `json:"arg_end"`

	// (50) env_start  %lu  (since Linux 3.5)  [PT]
	//        Address above which program environment is placed.
	// EnvStart uint64 `json:"env_start"`

	// (51) env_end  %lu  (since Linux 3.5)  [PT]
	//        Address below which program environment is placed.
	// EnvEnd uint64 `json:"env_end"`

	// (52) exit_code  %d  (since Linux 3.5)  [PT]
	//        The thread's exit status in the form reported by waitpid(2).
	// ExitCode int64 `json:"exit_code"`
}

// read and parse `$procPath/$pid/stat` to read process cpu usage information from the kernel
func getCpuUsageInfo(procPath string, pid string) (ProcPidStat, error) {
	// File Format of `/proc/$pid/stat`
	// ===================================
	//
	// ```
	// 1280 (mt76-tx phy0) S 2 0 0 0 -1 2129984 0 0 0 0 0 661 0 0 -2 0 1 0 1759 0 0 18446744073709551615 0 0 0 0 0 0 0 2147483647 0 0 0 0 17 15 1 1 0 0 0 0 0 0 0 0 0 0 0
	// ```
	//
	// Parsing Rules
	// =============
	//
	// - values are separated by spaces
	// - between '(' and ')' can be additional spaces which belong to the same value

	processPath := filepath.Join(procPath, pid)
	deviceInfoPath := filepath.Join(processPath, "stat")

	f, err := os.Open(deviceInfoPath)
	if err != nil {
		return ProcPidStat{}, err
	}

	// a common size encountered is about 350 bytes but can vary largely between processes
	// this buffer size is a guess to be *always* larger in *any* environment than what the `/proc/$pid/stat` contains
	dataMaxLen := 5 * 1024

	data := make([]byte, dataMaxLen)
	bytesRead, err := f.Read(data)
	if err != nil {
		return ProcPidStat{}, err
	}
	if bytesRead >= dataMaxLen {
		return ProcPidStat{}, fmt.Errorf("buffersize(%d) exhausted while reading File(%s)", dataMaxLen, deviceInfoPath)
	}
	data = data[0 : bytesRead-1]

	statcontent := string(data)

	dataPieces := make([]string, 0, 52)
	elementIdx := 0
	stringCapture := false
	value := ""
	for _, charRune := range statcontent {
		char := string(charRune)
		if char == " " && !stringCapture {
			dataPieces = append(dataPieces, value)
			value = ""
			elementIdx += 1
			continue
		}
		if char == "(" {
			stringCapture = true
		}
		if char == ")" {
			stringCapture = false
		}
		value += char
	}
	dataPieces = append(dataPieces, value)

	asInt64 := func(val string) int64 {
		intval, err := strconv.ParseInt(val, 10, 64)
		assert.Assert(err == nil, "val is expected to be an int64", val, err)
		return intval
	}

	asUint64 := func(val string) uint64 {
		intval, err := strconv.ParseUint(val, 10, 64)
		assert.Assert(err == nil, "val is expected to be an uint64", val, err)
		return intval
	}

	info := ProcPidStat{}
	info.Pid = asInt64(dataPieces[0])
	info.Comm = dataPieces[1]
	// info.State = dataPieces[2]
	// info.Ppid = asInt64(dataPieces[3])
	// info.Pgrp = asInt64(dataPieces[4])
	// info.Session = asInt64(dataPieces[5])
	// info.TtyNr = asInt64(dataPieces[6])
	// info.Tpgid = asInt64(dataPieces[7])
	// info.Flags = asUint64(dataPieces[8])
	// info.Minflt = asUint64(dataPieces[9])
	// info.Cminflt = asUint64(dataPieces[10])
	// info.Majflt = asUint64(dataPieces[11])
	// info.Cmajflt = asUint64(dataPieces[12])
	info.Utime = asUint64(dataPieces[13])
	info.Stime = asUint64(dataPieces[14])
	info.Cutime = asUint64(dataPieces[15])
	info.Cstime = asUint64(dataPieces[16])
	// info.Priority = asInt64(dataPieces[17])
	// info.Nice = asInt64(dataPieces[18])
	// info.NumThreads = asInt64(dataPieces[19])
	// info.Itrealvalue = asInt64(dataPieces[20])
	info.Starttime = asUint64(dataPieces[21])
	// info.Vsize = asUint64(dataPieces[22])
	// info.Rss = asInt64(dataPieces[23])
	// info.Rsslim = asUint64(dataPieces[24])
	// info.Startcode = asUint64(dataPieces[25])
	// info.Endcode = asUint64(dataPieces[26])
	// info.Startstack = asUint64(dataPieces[27])
	// info.Kstkesp = asUint64(dataPieces[28])
	// info.Kstkeip = asUint64(dataPieces[29])
	// info.Signal = asUint64(dataPieces[30])
	// info.Blocked = asUint64(dataPieces[31])
	// info.Sigignore = asUint64(dataPieces[32])
	// info.Sigcatch = asUint64(dataPieces[33])
	// info.Wchan = asUint64(dataPieces[34])
	// info.Nswap = asUint64(dataPieces[35])
	// info.Cnswap = asUint64(dataPieces[36])
	// info.ExitSignal = asUint64(dataPieces[37])
	// info.Processor = asInt64(dataPieces[38])
	// info.RtPriority = asUint64(dataPieces[39])
	// info.Policy = asUint64(dataPieces[40])
	// info.DelayacctBlkioTicks = asUint64(dataPieces[41])
	// info.GuestTime = asUint64(dataPieces[42])
	// info.CguestTime = asInt64(dataPieces[43])
	// info.StartData = asUint64(dataPieces[44])
	// info.EndData = asUint64(dataPieces[45])
	// info.StartBrk = asUint64(dataPieces[46])
	// info.ArgStart = asUint64(dataPieces[47])
	// info.ArgEnd = asUint64(dataPieces[48])
	// info.EnvStart = asUint64(dataPieces[49])
	// info.EnvEnd = asUint64(dataPieces[50])
	// info.ExitCode = asInt64(dataPieces[51])

	return info, nil
}

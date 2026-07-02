//go:build windows

package main

import (
	"log"
	"syscall"
	"unsafe"
)

var (
	kernel32                     = syscall.NewLazyDLL("kernel32.dll")
	procCreateJobObjectW         = kernel32.NewProc("CreateJobObjectW")
	procSetInformationJobObject  = kernel32.NewProc("SetInformationJobObject")
	procAssignProcessToJobObject = kernel32.NewProc("AssignProcessToJobObject")
	procGetCurrentProcess        = kernel32.NewProc("GetCurrentProcess")
)

const (

	jobObjectExtendedLimitInformationClass = 9

	jobObjectLimitKillOnJobClose = 0x00002000
)

type ioCounters struct {
	ReadOperationCount  uint64
	WriteOperationCount uint64
	OtherOperationCount uint64
	ReadTransferCount   uint64
	WriteTransferCount  uint64
	OtherTransferCount  uint64
}

type jobObjectBasicLimitInformation struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
}

type jobObjectExtendedLimitInformation struct {
	BasicLimitInformation jobObjectBasicLimitInformation
	IoInfo                ioCounters
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}

var globalJobHandle syscall.Handle

func init() {
	setupProcessKillJob()
}

func setupProcessKillJob() {
	hJob, _, err := procCreateJobObjectW.Call(0, 0)
	if hJob == 0 {
		log.Printf("job: CreateJobObject failed: %v (child mpv processes will not be auto-killed)", err)
		return
	}
	job := syscall.Handle(hJob)

	var info jobObjectExtendedLimitInformation
	info.BasicLimitInformation.LimitFlags = jobObjectLimitKillOnJobClose

	r, _, err := procSetInformationJobObject.Call(
		uintptr(job),
		uintptr(jobObjectExtendedLimitInformationClass),
		uintptr(unsafe.Pointer(&info)),
		unsafe.Sizeof(info),
	)
	if r == 0 {
		log.Printf("job: SetInformationJobObject failed: %v", err)
		_ = syscall.CloseHandle(job)
		return
	}

	curProc, _, _ := procGetCurrentProcess.Call()
	r, _, err = procAssignProcessToJobObject.Call(uintptr(job), curProc)
	if r == 0 {
		log.Printf("job: AssignProcessToJobObject failed: %v", err)
		_ = syscall.CloseHandle(job)
		return
	}

	globalJobHandle = job
	log.Printf("job: kill-on-close job active — child mpv processes will die together with goplay")
}

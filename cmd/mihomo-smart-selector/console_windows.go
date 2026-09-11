//go:build windows

package main

import (
	"bufio"
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

// A double-click launch owns its console; preserve startup errors long enough
// to read. Shell launches, tests and redirected/service execution never pause.
func pauseOnLaunchError() {
	kernel := syscall.NewLazyDLL("kernel32.dll")
	var mode uint32
	ok, _, _ := kernel.NewProc("GetConsoleMode").Call(os.Stdin.Fd(), uintptr(unsafe.Pointer(&mode)))
	if ok == 0 {
		return
	}
	ok, _, _ = kernel.NewProc("GetConsoleMode").Call(os.Stderr.Fd(), uintptr(unsafe.Pointer(&mode)))
	if ok == 0 {
		return
	}
	var processes [2]uint32
	count, _, _ := kernel.NewProc("GetConsoleProcessList").Call(uintptr(unsafe.Pointer(&processes[0])), uintptr(len(processes)))
	if count != 1 {
		return
	}
	fmt.Fprintln(os.Stderr, "Press Enter to close this window...")
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
}

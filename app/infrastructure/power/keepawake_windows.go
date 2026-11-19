// Package power keeps Windows from sleeping while a batch is sending.
package power

import (
	"runtime"

	"golang.org/x/sys/windows"
)

const (
	esContinuous     = 0x80000000
	esSystemRequired = 0x00000001
)

var setThreadExecutionState = windows.NewLazySystemDLL("kernel32.dll").NewProc("SetThreadExecutionState")

// KeepAwake asks Windows not to sleep until the returned release function is
// called. The display may still turn off; only system sleep is blocked.
//
// SetThreadExecutionState is per OS thread: the request lives as long as the
// thread that made it. Go goroutines hop between OS threads, so the call is
// made from a dedicated goroutine pinned with LockOSThread, which then just
// waits for release — the same reason the Outlook COM worker pins its
// thread.
func KeepAwake() (release func()) {
	done := make(chan struct{})
	ready := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		setThreadExecutionState.Call(esContinuous | esSystemRequired)
		close(ready)
		<-done
		setThreadExecutionState.Call(esContinuous) // clear the request
	}()
	<-ready
	return func() { close(done) }
}

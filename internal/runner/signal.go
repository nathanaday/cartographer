package runner

import "syscall"

// sysSignal0 checks that a process exists without signalling it.
var sysSignal0 = syscall.Signal(0)

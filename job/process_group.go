package job

import (
	"syscall"

	"golang.org/x/sys/unix"
)

func Tcgetpgrp() int {
	pgrpid, _ := unix.IoctlGetInt(syscall.Stdin, unix.TIOCGPGRP)
	return pgrpid
}

func tcsetpgrp(pgrpid int) {
	unix.IoctlSetPointerInt(syscall.Stdin, unix.TIOCSPGRP, pgrpid)
}

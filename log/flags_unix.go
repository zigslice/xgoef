//go:build unix

package log

import "syscall"

func noFollowFlag() int { return syscall.O_NOFOLLOW }

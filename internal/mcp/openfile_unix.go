//go:build unix

package mcp

import (
	"errors"
	"syscall"
)

const openFlags = syscall.O_NOFOLLOW | syscall.O_NONBLOCK

func isSymlinkLoop(err error) bool { return errors.Is(err, syscall.ELOOP) }

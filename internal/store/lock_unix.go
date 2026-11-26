//go:build linux || darwin || freebsd || openbsd || netbsd || dragonfly

package store

import (
	"errors"
	"golang.org/x/sys/unix"
	"os"
)

func tryStateLock(f *os.File) (bool, error) {
	err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EINTR) {
		return false, nil
	}
	return err == nil, err
}

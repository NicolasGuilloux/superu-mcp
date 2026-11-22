//go:build !unix

package coursesu

import "syscall"

func detachAttr() *syscall.SysProcAttr {
	return nil
}

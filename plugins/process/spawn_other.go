//go:build !linux

package process

import (
	"os"
	"syscall"
)

func sysProcAttr() *syscall.SysProcAttr { return nil }

func defaultStart(exe string, argv []string) error {
	proc, err := os.StartProcess(exe, argv, &os.ProcAttr{
		Env:   os.Environ(),
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
		Sys:   sysProcAttr(),
	})
	if err != nil {
		return err
	}
	return proc.Release()
}

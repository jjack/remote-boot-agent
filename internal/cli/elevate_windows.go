//go:build windows

package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

func IsElevated() bool {
	var token windows.Token
	err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token)
	if err != nil {
		return false
	}
	defer token.Close()

	return token.IsElevated()
}

func ElevateAndApply(ctx context.Context, cfgFile string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		cwd = ""
	}

	verb := "runas"
	args := fmt.Sprintf("setup --apply --config \"%s\"", cfgFile)

	verbPtr, _ := syscall.UTF16PtrFromString(verb)
	exePtr, _ := syscall.UTF16PtrFromString(exe)
	argsPtr, _ := syscall.UTF16PtrFromString(args)
	cwdPtr, _ := syscall.UTF16PtrFromString(cwd)

	showCmd := int32(windows.SW_NORMAL)

	err = windows.ShellExecute(0, verbPtr, exePtr, argsPtr, cwdPtr, showCmd)
	if err != nil {
		return fmt.Errorf("failed to request elevation: %w", err)
	}

	return nil
}

func Elevate(args []string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		cwd = ""
	}

	verb := "runas"
	// Join args, escaping those that need it
	var escapedArgs []string
	for _, arg := range args {
		if strings.Contains(arg, " ") {
			escapedArgs = append(escapedArgs, fmt.Sprintf("\"%s\"", arg))
		} else {
			escapedArgs = append(escapedArgs, arg)
		}
	}
	argStr := strings.Join(escapedArgs, " ")

	verbPtr, _ := syscall.UTF16PtrFromString(verb)
	exePtr, _ := syscall.UTF16PtrFromString(exe)
	argsPtr, _ := syscall.UTF16PtrFromString(argStr)
	cwdPtr, _ := syscall.UTF16PtrFromString(cwd)

	showCmd := int32(windows.SW_NORMAL)

	err = windows.ShellExecute(0, verbPtr, exePtr, argsPtr, cwdPtr, showCmd)
	if err != nil {
		return fmt.Errorf("failed to request elevation: %w", err)
	}

	return nil
}

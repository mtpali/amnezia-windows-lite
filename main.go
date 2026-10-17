/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2019-2022 WireGuard LLC. All Rights Reserved.
 */

package main

import (
	"debug/pe"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/windows"

	"github.com/amnezia-vpn/amneziawg-windows/v3/tunnel"

	"github.com/mtpali/amnezia-windows-lite/elevate"
	"github.com/mtpali/amnezia-windows-lite/l18n"
	"github.com/mtpali/amnezia-windows-lite/manager"
	"github.com/mtpali/amnezia-windows-lite/portable"
	"github.com/mtpali/amnezia-windows-lite/ui"
)

func fatal(v ...any) {
	windows.MessageBox(0, windows.StringToUTF16Ptr(fmt.Sprint(v...)), windows.StringToUTF16Ptr(l18n.Sprintf("Error")), windows.MB_ICONERROR)
	os.Exit(1)
}

func fatalf(format string, v ...any) {
	fatal(l18n.Sprintf(format, v...))
}

func info(title, format string, v ...any) {
	windows.MessageBox(0, windows.StringToUTF16Ptr(l18n.Sprintf(format, v...)), windows.StringToUTF16Ptr(title), windows.MB_ICONINFORMATION)
}

func usage() {
	flags := [...]string{
		l18n.Sprintf("(no argument): elevate and install manager service"),
		"/installmanagerservice",
		"/installtunnelservice CONFIG_PATH",
		"/uninstallmanagerservice",
		"/uninstalltunnelservice TUNNEL_NAME",
		"/managerservice",
		"/tunnelservice CONFIG_PATH",
		"/ui CMD_READ_HANDLE CMD_WRITE_HANDLE CMD_EVENT_HANDLE",
	}
	builder := strings.Builder{}
	for _, flag := range flags {
		builder.WriteString(fmt.Sprintf("    %s\n", flag))
	}
	info(l18n.Sprintf("Command Line Options"), "Usage: %s [\n%s]", os.Args[0], builder.String())
	os.Exit(1)
}

func checkForWow64() {
	b, err := func() (bool, error) {
		var processMachine, nativeMachine uint16
		err := windows.IsWow64Process2(windows.CurrentProcess(), &processMachine, &nativeMachine)
		if err == nil {
			return processMachine != pe.IMAGE_FILE_MACHINE_UNKNOWN, nil
		}
		if !errors.Is(err, windows.ERROR_PROC_NOT_FOUND) {
			return false, err
		}
		var b bool
		err = windows.IsWow64Process(windows.CurrentProcess(), &b)
		if err != nil {
			return false, err
		}
		return b, nil
	}()
	if err != nil {
		fatalf("Unable to determine whether the process is running under WOW64: %v", err)
	}
	if b {
		fatalf("You must use the native version of amnezia on this computer.")
	}
}

func checkForAdminGroup() {
	// This is not a security check, but rather a user-confusion one.
	var processToken windows.Token
	err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY|windows.TOKEN_DUPLICATE, &processToken)
	if err != nil {
		fatalf("Unable to open current process token: %v", err)
	}
	defer processToken.Close()
	if !elevate.TokenIsElevatedOrElevatable(processToken) {
		fatalf("amnezia may only be used by users who are a member of the Builtin %s group.", elevate.AdminGroupName())
	}
}

func checkForAdminDesktop() {
	adminDesktop, err := elevate.IsAdminDesktop()
	if !adminDesktop && err == nil {
		fatalf("amnezia is running, but the UI is only accessible from desktops of the Builtin %s group.", elevate.AdminGroupName())
	}
}

func execElevatedManagerServiceInstaller() error {
	path, err := os.Executable()
	if err != nil {
		return err
	}
	err = elevate.ShellExecute(path, "/installmanagerservice", "", windows.SW_SHOW)
	if err != nil && err != windows.ERROR_CANCELLED {
		return err
	}
	os.Exit(0)
	return windows.ERROR_UNHANDLED_EXCEPTION // Not reached
}

func pipeFromHandleArgument(handleStr string) (*os.File, error) {
	handleInt, err := strconv.ParseUint(handleStr, 10, 64)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(handleInt), "pipe"), nil
}

func main() {
	// Dependencies may emit diagnostic messages; this client has no log output,
	// shared log mapping, or persistent log files. Errors still reach the UI.
	log.SetOutput(io.Discard)
	if windows.SetDllDirectory("") != nil || windows.SetDefaultDllDirectories(windows.LOAD_LIBRARY_SEARCH_SYSTEM32) != nil {
		panic("failed to restrict dll search path")
	}

	if err := portable.Configure(); err != nil {
		fatal(err)
	}
	checkForWow64()

	if len(os.Args) <= 1 {
		if ui.RaiseUI() {
			return
		}
		checkForAdminGroup()
		err := execElevatedManagerServiceInstaller()
		if err != nil {
			fatal(err)
		}
		return
	}
	switch os.Args[1] {
	case "/installmanagerservice":
		if len(os.Args) != 2 {
			usage()
		}
		go ui.WaitForRaiseUIThenQuit()
		err := manager.InstallManager()
		if err != nil {
			if err == manager.ErrManagerAlreadyRunning {
				checkForAdminDesktop()
			}
			fatal(err)
		}
		checkForAdminDesktop()
		time.Sleep(30 * time.Second)
		fatalf("amnezia system tray icon did not appear after 30 seconds.")
		return
	case "/uninstallmanagerservice":
		if len(os.Args) != 2 {
			usage()
		}
		err := manager.UninstallManager()
		if err != nil {
			fatal(err)
		}
		return
	case "/managerservice":
		if len(os.Args) != 2 {
			usage()
		}
		err := manager.Run()
		if err != nil {
			fatal(err)
		}
		return
	case "/installtunnelservice":
		if len(os.Args) != 3 {
			usage()
		}
		err := manager.InstallTunnel(os.Args[2])
		if err != nil {
			fatal(err)
		}
		return
	case "/uninstalltunnelservice":
		if len(os.Args) != 3 {
			usage()
		}
		err := manager.UninstallTunnel(os.Args[2])
		if err != nil {
			fatal(err)
		}
		return
	case "/tunnelservice":
		if len(os.Args) != 3 {
			usage()
		}
		err := tunnel.Run(os.Args[2])
		if err != nil {
			fatal(err)
		}
		return
	case "/ui":
		if len(os.Args) != 5 {
			usage()
		}
		var processToken windows.Token
		isAdmin := false
		err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY|windows.TOKEN_DUPLICATE, &processToken)
		if err == nil {
			isAdmin = elevate.TokenIsElevatedOrElevatable(processToken)
			processToken.Close()
		}
		if isAdmin {
			err := elevate.DropAllPrivileges(false)
			if err != nil {
				fatal(err)
			}
		}
		readPipe, err := pipeFromHandleArgument(os.Args[2])
		if err != nil {
			fatal(err)
		}
		writePipe, err := pipeFromHandleArgument(os.Args[3])
		if err != nil {
			fatal(err)
		}
		eventPipe, err := pipeFromHandleArgument(os.Args[4])
		if err != nil {
			fatal(err)
		}
		manager.InitializeIPCClient(readPipe, writePipe, eventPipe)
		ui.IsAdmin = isAdmin
		ui.RunUI()
		return
	}
	usage()
}

/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2019-2022 WireGuard LLC. All Rights Reserved.
 */

package ui

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/lxn/walk"
	"github.com/lxn/win"
	"golang.org/x/sys/windows"

	"github.com/mtpali/amnezia-windows-lite/internal/preferences"
	"github.com/mtpali/amnezia-windows-lite/l18n"
	"github.com/mtpali/amnezia-windows-lite/manager"
	"github.com/mtpali/amnezia-windows-lite/version"
)

var (
	noTrayAvailable              = false
	shouldQuitManagerWhenExiting = false
	startTime                    = time.Now()
	IsAdmin                      = false // A global, because this really is global for the process
)

func RunUI() {
	runtime.LockOSThread()
	initOLED()
	uiPreferences = preferences.Load(preferencesPath())
	windows.SetProcessPriorityBoost(windows.CurrentProcess(), false)
	defer func() {
		if err := recover(); err != nil {
			showErrorCustom(nil, "Panic", fmt.Sprint(err, "\n\n", string(debug.Stack())))
			panic(err)
		}
	}()

	var (
		err  error
		mtw  *ManageTunnelsWindow
		tray *Tray
	)

	for mtw == nil {
		mtw, err = NewManageTunnelsWindow()
		if err != nil {
			time.Sleep(time.Millisecond * 400)
		}
	}

	for tray == nil {
		tray, err = NewTray(mtw)
		if err != nil {
			if version.OsIsCore() {
				noTrayAvailable = true
				break
			}
			time.Sleep(time.Millisecond * 400)
		}
	}

	manager.IPCClientRegisterManagerStopping(func() {
		mtw.Synchronize(func() {
			walk.App().Exit(0)
		})
	})

	if tray == nil {
		win.ShowWindow(mtw.Handle(), win.SW_MINIMIZE)
	}

	mtw.Show()
	mtw.Run()
	if tray != nil {
		tray.Dispose()
	}
	mtw.Dispose()

	if shouldQuitManagerWhenExiting {
		_, err := manager.IPCClientQuit(true)
		if err != nil {
			showErrorCustom(nil, l18n.Sprintf("Error Exiting Windows Lite"), l18n.Sprintf("Unable to exit service due to: %v. You may want to stop Windows Lite from the service manager.", err))
		}
	}
}

func onQuit() {
	shouldQuitManagerWhenExiting = true
	walk.App().Exit(0)
}

func showError(err error, owner walk.Form) bool {
	if err == nil {
		return false
	}

	showErrorCustom(owner, l18n.Sprintf("Error"), err.Error())

	return true
}

func showErrorCustom(owner walk.Form, title, message string) {
	oledMessageBox(owner, title, message, walk.MsgBoxIconError)
}

func showWarningCustom(owner walk.Form, title, message string) {
	oledMessageBox(owner, title, message, walk.MsgBoxIconWarning)
}

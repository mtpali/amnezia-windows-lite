/* SPDX-License-Identifier: MIT
 * Copyright (C) 2019-2022 WireGuard LLC. All Rights Reserved.
 */
package ui

import (
	"github.com/lxn/walk"
	"github.com/lxn/win"
	"github.com/mtpali/amnezia-windows-lite/manager"
	"golang.org/x/sys/windows"
	"sync"
)

type ManageTunnelsWindow struct {
	walk.FormBase
	tunnelsPage     *TunnelsPage
	tunnelChangedCB *manager.TunnelChangeCallback
	anchoring       bool
}

const (
	manageWindowWindowClass = "Windows Lite - Portable"
	raiseMsg                = win.WM_USER + 0x3510
)

var taskbarButtonCreatedMsg uint32
var initedManageTunnels sync.Once

func NewManageTunnelsWindow() (*ManageTunnelsWindow, error) {
	initedManageTunnels.Do(func() {
		walk.AppendToWalkInit(func() {
			walk.MustRegisterWindowClass(manageWindowWindowClass)
			taskbarButtonCreatedMsg = win.RegisterWindowMessage(windows.StringToUTF16Ptr("TaskbarButtonCreated"))
		})
	})
	mtw := new(ManageTunnelsWindow)
	if err := walk.InitWindow(mtw, nil, manageWindowWindowClass, win.WS_OVERLAPPEDWINDOW, win.WS_EX_CONTROLPARENT); err != nil {
		return nil, err
	}
	ok := false
	defer func() {
		if !ok {
			mtw.Dispose()
		}
	}()
	mtw.SetName("amnezia")
	mtw.SetTitle("amnezia")
	font, err := walk.NewFont("Segoe UI", 9, 0)
	if err != nil {
		return nil, err
	}
	mtw.AddDisposable(font)
	mtw.SetFont(font)
	mtw.SetSize(walk.Size{480, 310})
	mtw.SetMinMaxSize(walk.Size{440, 290}, walk.Size{})
	layout := walk.NewVBoxLayout()
	layout.SetMargins(walk.Margins{10, 10, 10, 10})
	layout.SetSpacing(6)
	mtw.SetLayout(layout)
	if icon, err := loadLogoIcon(32); err == nil {
		mtw.SetIcon(icon)
	}
	win.ChangeWindowMessageFilterEx(mtw.Handle(), raiseMsg, win.MSGFLT_ALLOW, nil)
	mtw.tunnelsPage, err = NewTunnelsPage(mtw)
	if err != nil {
		return nil, err
	}
	if err = mtw.tunnelsPage.CreateToolbar(); err != nil {
		return nil, err
	}
	mtw.Closing().Attach(func(canceled *bool, reason walk.CloseReason) {
		*canceled = true
		if !noTrayAvailable {
			mtw.Hide()
		} else {
			onQuit()
		}
	})
	mtw.VisibleChanged().Attach(func() {
		if mtw.Visible() {
			mtw.tunnelsPage.updateConfView()
		}
	})
	mtw.tunnelChangedCB = manager.IPCClientRegisterTunnelChange(func(t *manager.Tunnel, state, global manager.TunnelState, err error) {
		mtw.Synchronize(func() {
			if pi := mtw.ProgressIndicator(); pi != nil {
				if global == manager.TunnelStarting || global == manager.TunnelStopping {
					pi.SetState(walk.PIIndeterminate)
				} else {
					pi.SetState(walk.PINoProgress)
				}
			}
			if err != nil && mtw.Visible() {
				showErrorCustom(mtw, "Connection", err.Error())
			}
		})
	})
	applyOLED(mtw)
	mtw.Starting().Attach(func() { mtw.placeBottomRight(false) })
	ok = true
	return mtw, nil
}
func (mtw *ManageTunnelsWindow) Dispose() {
	if mtw.tunnelChangedCB != nil {
		mtw.tunnelChangedCB.Unregister()
		mtw.tunnelChangedCB = nil
	}
	mtw.FormBase.Dispose()
}
func (mtw *ManageTunnelsWindow) WndProc(hwnd win.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case win.WM_QUERYENDSESSION:
		return win.TRUE
	case raiseMsg:
		if mtw.tunnelsPage != nil {
			mtw.tunnelsPage.listView.SelectFirstActiveTunnel()
		}
		mtw.ShowAtBottomRight()
		raise(hwnd)
		return 0
	case win.WM_DPICHANGED, win.WM_DISPLAYCHANGE, win.WM_SETTINGCHANGE:
		result := mtw.FormBase.WndProc(hwnd, msg, wParam, lParam)
		if mtw.Visible() && !mtw.anchoring && (msg != win.WM_SETTINGCHANGE || wParam == 0x002F /* SPI_SETWORKAREA */) {
			mtw.placeBottomRight(false)
		}
		return result
	}
	return mtw.FormBase.WndProc(hwnd, msg, wParam, lParam)
}

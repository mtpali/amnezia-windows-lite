// SPDX-License-Identifier: MIT
package ui

import (
	"unsafe"

	"github.com/lxn/win"
	"github.com/mtpali/amnezia-windows-lite/internal/windowplacement"
	"golang.org/x/sys/windows"
)

var monitorFromRect = windows.NewLazySystemDLL("user32.dll").NewProc("MonitorFromRect")

func windowWorkArea(hwnd win.HWND, nearCursor bool) (win.RECT, bool) {
	monitor := win.MonitorFromWindow(hwnd, win.MONITOR_DEFAULTTONEAREST)
	if nearCursor {
		var point win.POINT
		if win.GetCursorPos(&point) {
			rect := win.RECT{Left: point.X, Top: point.Y, Right: point.X + 1, Bottom: point.Y + 1}
			if handle, _, _ := monitorFromRect.Call(uintptr(unsafe.Pointer(&rect)), win.MONITOR_DEFAULTTONEAREST); handle != 0 {
				monitor = win.HMONITOR(handle)
			}
		}
	}
	info := win.MONITORINFO{CbSize: uint32(unsafe.Sizeof(win.MONITORINFO{}))}
	if !win.GetMonitorInfo(monitor, &info) || info.RcWork.Right <= info.RcWork.Left || info.RcWork.Bottom <= info.RcWork.Top {
		return win.RECT{}, false
	}
	return info.RcWork, true
}

func (mtw *ManageTunnelsWindow) placeBottomRight(nearCursor bool) {
	if mtw.anchoring || win.IsIconic(mtw.Handle()) || win.IsZoomed(mtw.Handle()) {
		return
	}
	work, ok := windowWorkArea(mtw.Handle(), nearCursor)
	if !ok {
		return
	}
	mtw.anchoring = true
	defer func() { mtw.anchoring = false }()
	// Moving between monitors can synchronously change DPI and the window size.
	// Re-read physical bounds after that change before aligning the final edge.
	for i := 0; i < 2; i++ {
		bounds := mtw.BoundsPixels()
		x, y := windowplacement.BottomRight(int(work.Left), int(work.Top), int(work.Right), int(work.Bottom), bounds.Width, bounds.Height)
		win.SetWindowPos(mtw.Handle(), 0, int32(x), int32(y), 0, 0, win.SWP_NOSIZE|win.SWP_NOZORDER|win.SWP_NOACTIVATE)
	}
}

func (mtw *ManageTunnelsWindow) ShowAtBottomRight() {
	if win.IsIconic(mtw.Handle()) || win.IsZoomed(mtw.Handle()) {
		win.ShowWindow(mtw.Handle(), win.SW_RESTORE)
	}
	mtw.placeBottomRight(true)
	mtw.Show()
	mtw.placeBottomRight(false)
}

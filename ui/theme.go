// SPDX-License-Identifier: MIT
package ui

import (
	"github.com/lxn/walk"
	"github.com/lxn/win"
	"golang.org/x/sys/windows"
	"sync"
	"syscall"
	"unsafe"
)

var (
	oledBlack      = walk.RGB(0, 0, 0)
	oledText       = walk.RGB(232, 232, 232)
	oledMuted      = walk.RGB(155, 155, 155)
	oledAccent     = walk.RGB(115, 245, 178)
	oledSelection  = walk.RGB(25, 25, 25)
	oledBrush      *walk.SolidColorBrush
	oledOnce       sync.Once
	oledControls   = make(map[win.HWND]bool)
	oledSubclass   uintptr
	themeDLL       = windows.NewLazySystemDLL("uxtheme.dll")
	setWindowTheme = themeDLL.NewProc("SetWindowTheme")
	dwmAttribute   = windows.NewLazySystemDLL("dwmapi.dll").NewProc("DwmSetWindowAttribute")
	comctl         = windows.NewLazySystemDLL("comctl32.dll")
	setSubclass    = comctl.NewProc("SetWindowSubclass")
	removeSubclass = comctl.NewProc("RemoveWindowSubclass")
	defSubclass    = comctl.NewProc("DefSubclassProc")
)

func initOLED() {
	oledOnce.Do(func() {
		oledSubclass = syscall.NewCallback(oledControlProc)
		oledBrush, _ = walk.NewSolidColorBrush(oledBlack)
		// ForceDark for native menus and common dialogs, available since 1903.
		if windows.RtlGetVersion().BuildNumber >= 18362 {
			if module, err := windows.LoadLibrary("uxtheme.dll"); err == nil {
				defer windows.FreeLibrary(module)
				if proc, err := windows.GetProcAddressByOrdinal(module, 135); err == nil {
					syscall.SyscallN(proc, 2)
				}
				if proc, err := windows.GetProcAddressByOrdinal(module, 136); err == nil {
					syscall.SyscallN(proc)
				}
			}
		}
	})
}
func applyOLED(w walk.Window) {
	if w == nil {
		return
	}
	initOLED()
	if oledBrush != nil {
		w.AsWindowBase().SetBackground(oledBrush)
	}
	hwnd := w.Handle()
	if _, ok := w.(walk.Form); ok {
		for _, attr := range []uint32{19, 20} {
			v := uint32(1)
			dwmAttribute.Call(uintptr(hwnd), uintptr(attr), uintptr(unsafe.Pointer(&v)), 4)
		}
		for _, pair := range [][2]uint32{{34, uint32(walk.RGB(40, 40, 40))}, {35, 0}, {36, uint32(oledText)}} {
			value := pair[1]
			dwmAttribute.Call(uintptr(hwnd), uintptr(pair[0]), uintptr(unsafe.Pointer(&value)), 4)
		}
	}
	theme := windows.StringToUTF16Ptr("DarkMode_Explorer")
	setWindowTheme.Call(uintptr(hwnd), uintptr(unsafe.Pointer(theme)), 0)
	switch control := w.(type) {
	case *walk.Label:
		control.SetTextColor(oledText)
	case *walk.TextLabel:
		control.SetTextColor(oledText)
	case *walk.LineEdit:
		control.SetTextColor(oledText)
	case *walk.TextEdit:
		control.SetTextColor(oledText)
	case *walk.PushButton:
		oledControls[hwnd] = false
		setSubclass.Call(uintptr(hwnd), oledSubclass, 1, 0)
	case *walk.CheckBox:
		oledControls[hwnd] = true
		setSubclass.Call(uintptr(hwnd), oledSubclass, 1, 0)
	case *walk.TableView:
		win.SendMessage(hwnd, win.LVM_SETBKCOLOR, 0, uintptr(oledBlack))
		win.SendMessage(hwnd, win.LVM_SETTEXTBKCOLOR, 0, uintptr(oledBlack))
		win.SendMessage(hwnd, win.LVM_SETTEXTCOLOR, 0, uintptr(oledText))
	}
	if container, ok := w.(walk.Container); ok {
		for i := 0; i < container.Children().Len(); i++ {
			applyOLED(container.Children().At(i))
		}
	}
	w.Invalidate()
}
func oledControlProc(hwnd win.HWND, msg uint32, wParam, lParam, id, data uintptr) uintptr {
	switch msg {
	case win.WM_ERASEBKGND:
		return 1
	case win.WM_PAINT:
		var ps win.PAINTSTRUCT
		dc := win.BeginPaint(hwnd, &ps)
		var rect win.RECT
		win.GetClientRect(hwnd, &rect)
		drawOLEDControl(hwnd, dc, rect, oledControls[hwnd])
		win.EndPaint(hwnd, &ps)
		return 0
	case win.WM_NCDESTROY:
		delete(oledControls, hwnd)
		removeSubclass.Call(uintptr(hwnd), oledSubclass, id)
	}
	result, _, _ := defSubclass.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return result
}
func fillOLED(dc win.HDC, r *win.RECT, c walk.Color) {
	brush := win.CreateBrushIndirect(&win.LOGBRUSH{LbStyle: win.BS_SOLID, LbColor: win.COLORREF(c)})
	fillRectProc.Call(uintptr(dc), uintptr(unsafe.Pointer(r)), uintptr(brush))
	win.DeleteObject(win.HGDIOBJ(brush))
}
func drawOLEDControl(hwnd win.HWND, dc win.HDC, rect win.RECT, checkbox bool) {
	text := make([]uint16, int(win.SendMessage(hwnd, win.WM_GETTEXTLENGTH, 0, 0))+1)
	win.SendMessage(hwnd, win.WM_GETTEXT, uintptr(len(text)), uintptr(unsafe.Pointer(&text[0])))
	old := win.SelectObject(dc, win.HGDIOBJ(win.SendMessage(hwnd, win.WM_GETFONT, 0, 0)))
	defer win.SelectObject(dc, old)
	win.SetBkMode(dc, win.TRANSPARENT)
	color := oledText
	if !win.IsWindowEnabled(hwnd) {
		color = oledMuted
	}
	win.SetTextColor(dc, win.COLORREF(color))
	border := walk.RGB(49, 49, 49)
	if win.GetFocus() == hwnd {
		border = oledAccent
	}
	if checkbox {
		fillOLED(dc, &rect, oledBlack)
		size := int32(15)
		if rect.Bottom > 30 {
			size = rect.Bottom / 2
		}
		box := win.RECT{Left: 0, Top: (rect.Bottom - size) / 2, Right: size, Bottom: (rect.Bottom + size) / 2}
		fillOLED(dc, &box, border)
		inner := box
		inner.Left++
		inner.Top++
		inner.Right--
		inner.Bottom--
		fillOLED(dc, &inner, oledBlack)
		if win.SendMessage(hwnd, win.BM_GETCHECK, 0, 0) == win.BST_CHECKED {
			drawOLEDText(dc, windows.StringToUTF16Ptr("✓"), -1, &box, win.DT_CENTER|win.DT_VCENTER|win.DT_SINGLELINE)
		}
		rect.Left = size + 7
		drawOLEDText(dc, &text[0], -1, &rect, win.DT_LEFT|win.DT_VCENTER|win.DT_SINGLELINE)
	} else {
		fillOLED(dc, &rect, border)
		rect.Left++
		rect.Top++
		rect.Right--
		rect.Bottom--
		face := walk.RGB(18, 18, 18)
		if win.SendMessage(hwnd, win.BM_GETSTATE, 0, 0)&win.BST_PUSHED != 0 {
			face = walk.RGB(38, 38, 38)
		}
		fillOLED(dc, &rect, face)
		drawOLEDText(dc, &text[0], -1, &rect, win.DT_CENTER|win.DT_VCENTER|win.DT_SINGLELINE|win.DT_END_ELLIPSIS)
	}
}

type oledCellStyler struct{}

func (oledCellStyler) StyleCell(style *walk.CellStyle) {
	style.BackgroundColor = oledBlack
	style.TextColor = oledText
}

func drawOLEDText(dc win.HDC, text *uint16, n int32, rect *win.RECT, flags uint32) {
	win.DrawTextEx(dc, text, n, rect, flags, nil)
}

var fillRectProc = windows.NewLazySystemDLL("user32.dll").NewProc("FillRect")

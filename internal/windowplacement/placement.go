// SPDX-License-Identifier: MIT
package windowplacement

// BottomRight returns a window origin in physical pixels. Work areas already
// exclude the taskbar, including taskbars on the top or either side. Keep an
// oversized window's title bar reachable on a small or disconnected display.
func BottomRight(left, top, right, bottom, width, height int) (x, y int) {
	return max(left, right-width), max(top, bottom-height)
}

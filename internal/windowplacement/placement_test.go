// SPDX-License-Identifier: MIT
package windowplacement

import "testing"

func TestBottomRight(t *testing.T) {
	for _, tc := range []struct {
		name                        string
		left, top, right, bottom    int
		width, height, wantX, wantY int
	}{
		{"bottom taskbar", 0, 0, 1920, 1030, 535, 355, 1385, 675},
		{"left taskbar", 48, 0, 1920, 1080, 480, 310, 1440, 770},
		{"top taskbar", 0, 48, 1920, 1080, 480, 310, 1440, 770},
		{"right taskbar", 0, 0, 1872, 1080, 480, 310, 1392, 770},
		{"negative secondary monitor", -1920, -200, 0, 840, 480, 310, -480, 530},
		{"scaled display", 1920, 0, 4480, 1380, 720, 465, 3760, 915},
		{"oversized window", 0, 40, 320, 240, 480, 310, 0, 40},
	} {
		t.Run(tc.name, func(t *testing.T) {
			x, y := BottomRight(tc.left, tc.top, tc.right, tc.bottom, tc.width, tc.height)
			if x != tc.wantX || y != tc.wantY {
				t.Fatalf("got (%d, %d), want (%d, %d)", x, y, tc.wantX, tc.wantY)
			}
		})
	}
}

package ui

import (
	"encoding/gob"
	"github.com/amnezia-vpn/amneziawg-windows/v3/conf"
	"github.com/lxn/walk"
	"github.com/mtpali/amnezia-windows-lite/manager"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// Exercise the actual native window without UAC, driver installation, real
// configurations, or networking. The in-memory RPC server is read-only.
func TestLiteWindowStarts(t *testing.T) {
	if os.Getenv("LITE_UI_SMOKE") != "1" {
		t.Skip("enable LITE_UI_SMOKE to render the native UI")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	initOLED()
	requestR, requestW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	responseR, responseW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	eventsR, eventsW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		for _, f := range []*os.File{requestR, requestW, responseR, responseW, eventsR, eventsW} {
			f.Close()
		}
	}()
	go func() {
		decoder, encoder := gob.NewDecoder(requestR), gob.NewEncoder(responseW)
		for {
			var method manager.MethodType
			if decoder.Decode(&method) != nil {
				return
			}
			switch method {
			case manager.TunnelsMethodType:
				encoder.Encode([]manager.Tunnel{{Name: "Alpha"}, {Name: "Zulu"}})
				encoder.Encode("")
			case manager.StateMethodType:
				var name string
				decoder.Decode(&name)
				encoder.Encode(manager.TunnelStopped)
				encoder.Encode("")
			case manager.GlobalStateMethodType:
				encoder.Encode(manager.TunnelStopped)
				encoder.Encode("")
			case manager.StoredConfigMethodType, manager.RuntimeConfigMethodType:
				var name string
				decoder.Decode(&name)
				encoder.Encode(conf.Config{Name: name, Peers: []conf.Peer{{Endpoint: conf.Endpoint{Host: "192.0.2.1", Port: 51820}}}})
				encoder.Encode("")
			default:
				return
			}
		}
	}()
	manager.InitializeIPCClient(responseR, requestW, eventsR)
	IsAdmin = true
	uiPreferences.LastTunnel = "Zulu"
	window, err := NewManageTunnelsWindow()
	if err != nil {
		t.Fatal(err)
	}
	defer window.Dispose()
	if selected := window.tunnelsPage.listView.CurrentTunnel(); selected == nil || selected.Name != "Zulu" {
		t.Fatal("native list did not restore the last connection")
	}
	rendered := false
	window.Starting().Attach(func() {
		time.AfterFunc(500*time.Millisecond, func() {
			window.Synchronize(func() {
				defer walk.App().Exit(0)
				rendered = true
				bitmap, err := walk.NewBitmapFromWindow(window)
				if err != nil {
					t.Error(err)
					return
				}
				defer bitmap.Dispose()
				image, err := bitmap.ToImage()
				if err != nil {
					t.Error(err)
					return
				}
				// GDI compatible bitmaps do not define alpha. Treat every pixel
				// as opaque before PNG encoding, which would otherwise erase text.
				for i := 3; i < len(image.Pix); i += 4 {
					image.Pix[i] = 255
				}
				black, total, textPixels := 0, 0, 0
				bounds := image.Bounds()
				for y := 35; y < bounds.Dy()-8; y += 3 {
					for x := 8; x < bounds.Dx()-8; x += 3 {
						r, g, b, _ := image.At(x, y).RGBA()
						total++
						if r > 170*257 && g > 170*257 && b > 170*257 {
							textPixels++
						}
						if r < 30*257 && g < 30*257 && b < 30*257 {
							black++
						}
					}
				}
				if total == 0 || black*100 < total*85 {
					t.Errorf("OLED background is not predominantly black: %d/%d", black, total)
				}
				buttonBounds := window.tunnelsPage.confView.connect.BoundsPixels()
				if textPixels < 75 || buttonBounds.Width < 50 || buttonBounds.Height < 15 || !window.tunnelsPage.confView.connect.Visible() {
					t.Errorf("window contents were not rendered: text=%d button=%+v", textPixels, buttonBounds)
				}
				if dir := os.Getenv("LITE_SCREENSHOT_DIR"); dir != "" {
					if err := os.MkdirAll(dir, 0755); err != nil {
						t.Error(err)
						return
					}
					f, err := os.Create(filepath.Join(dir, "windows-lite.png"))
					if err != nil {
						t.Error(err)
						return
					}
					if err = png.Encode(f, image); err != nil {
						t.Error(err)
					}
					f.Close()
				}
			})
		})
	})
	window.Show()
	window.Run()
	if !rendered {
		t.Fatal("window did not run its message loop")
	}
}

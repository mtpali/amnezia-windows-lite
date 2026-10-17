package ui

import (
	"encoding/gob"
	"github.com/amnezia-vpn/amneziawg-windows/v3/conf"
	"github.com/lxn/walk"
	"github.com/lxn/win"
	"github.com/mtpali/amnezia-windows-lite/manager"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
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
	var liveSample atomic.Uint32
	liveSample.Store(1)
	go func() {
		decoder, encoder := gob.NewDecoder(requestR), gob.NewEncoder(responseW)
		encode := func(v any) {
			if err := encoder.Encode(v); err != nil {
				t.Errorf("fake manager could not encode %T: %v", v, err)
			}
		}
		for {
			var method manager.MethodType
			if decoder.Decode(&method) != nil {
				return
			}
			switch method {
			case manager.TunnelsMethodType:
				encode([]manager.Tunnel{{Name: "Alpha"}, {Name: "Zulu"}})
				encode("")
			case manager.StateMethodType:
				var name string
				decoder.Decode(&name)
				encode(manager.TunnelStarted)
				encode("")
			case manager.GlobalStateMethodType:
				encode(manager.TunnelStarted)
				encode("")
			case manager.StoredConfigMethodType, manager.RuntimeConfigMethodType:
				var name string
				decoder.Decode(&name)
				cfg := conf.Config{Name: name, Peers: []conf.Peer{
					{Endpoint: conf.Endpoint{Host: "192.0.2.1", Port: 51820}},
					{Endpoint: conf.Endpoint{Host: "192.0.2.2", Port: 51820}},
				}}
				if method == manager.RuntimeConfigMethodType {
					sample := liveSample.Load()
					if sample == 3 {
						encode(conf.Config{})
						encode("runtime statistics temporarily unavailable")
						continue
					}
					for i := range cfg.Peers {
						cfg.Peers[i].RxBytes = 512 * 1024
						cfg.Peers[i].TxBytes = 512
						if sample == 2 {
							cfg.Peers[i].RxBytes = 2 * 1024 * 1024
							cfg.Peers[i].TxBytes = 1024
						}
						cfg.Peers[i].LastHandshakeTime = conf.HandshakeTime(time.Now().Add(-5 * time.Second).UnixNano())
					}
				}
				encode(cfg)
				encode("")
			default:
				return
			}
		}
	}()
	manager.InitializeIPCClient(responseR, requestW, eventsR)
	initial, err := (&manager.Tunnel{Name: "Zulu"}).RuntimeConfig()
	if err != nil {
		t.Fatal("initial live RPC failed", err)
	}
	if len(initial.Peers) != 2 || initial.Peers[0].RxBytes != 512*1024 {
		t.Fatalf("initial live RPC lost counters: %+v", initial.Peers)
	}
	IsAdmin = true
	uiPreferences.LastTunnel = "Zulu"
	window, err := NewManageTunnelsWindow()
	if err != nil {
		t.Fatal(err)
	}
	defer window.Dispose()
	if window.Title() != "amnezia" {
		t.Fatalf("unexpected application title: %q", window.Title())
	}
	if selected := window.tunnelsPage.listView.CurrentTunnel(); selected == nil || selected.Name != "Zulu" {
		t.Fatal("native list did not restore the last connection")
	}
	rendered := false
	waitForTraffic := func(want string, next func()) {
		deadline := time.Now().Add(5 * time.Second)
		var poll func()
		poll = func() {
			if got := window.tunnelsPage.confView.traffic.Text(); got == want {
				next()
				return
			} else if time.Now().After(deadline) {
				t.Errorf("traffic label did not update: got %q, want %q", got, want)
				walk.App().Exit(1)
				return
			}
			time.AfterFunc(25*time.Millisecond, func() { window.Synchronize(poll) })
		}
		poll()
	}
	capture := func() {
		defer walk.App().Exit(0)
		rendered = true
		checkPlacement := func() {
			work, ok := windowWorkArea(window.Handle(), false)
			bounds := window.BoundsPixels()
			if !ok || bounds.X+bounds.Width != int(work.Right) || bounds.Y+bounds.Height != int(work.Bottom) {
				t.Errorf("window is not at bottom right above the taskbar: bounds=%+v work=%+v", bounds, work)
			}
		}
		checkPlacement()
		// Reopening from the tray must restore the anchor even if the
		// window was previously dragged somewhere else.
		window.Hide()
		win.SetWindowPos(window.Handle(), 0, 20, 20, 0, 0, win.SWP_NOSIZE|win.SWP_NOZORDER|win.SWP_NOACTIVATE)
		window.ShowAtBottomRight()
		checkPlacement()
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
		if textPixels < 25 || buttonBounds.Width < 50 || buttonBounds.Height < 15 || !win.IsWindowVisible(window.tunnelsPage.confView.connect.Handle()) {
			t.Errorf("window contents were not rendered: text=%d button=%+v", textPixels, buttonBounds)
		}
		if dir := os.Getenv("LITE_SCREENSHOT_DIR"); dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Error(err)
				return
			}
			f, err := os.Create(filepath.Join(dir, "amnezia.png"))
			if err != nil {
				t.Error(err)
				return
			}
			if err = png.Encode(f, image); err != nil {
				t.Error(err)
			}
			f.Close()
		}
	}
	window.Starting().Attach(func() {
		waitForTraffic("↓ 1.0 MB   ↑ 1.0 KB", func() {
			liveSample.Store(2)
			// No forced refresh here: the normal polling loop must fetch
			// the larger live counters instead of the zeroed saved config.
			waitForTraffic("↓ 4.0 MB   ↑ 2.0 KB", func() {
				liveSample.Store(3)
				window.tunnelsPage.confView.refresh()
				waitForTraffic("↓ —   ↑ —", func() {
					liveSample.Store(2)
					window.tunnelsPage.confView.refresh()
					waitForTraffic("↓ 4.0 MB   ↑ 2.0 KB", capture)
				})
			})
		})
	})
	window.ShowAtBottomRight()
	window.Run()
	if !rendered {
		t.Fatal("window did not run its message loop")
	}
}

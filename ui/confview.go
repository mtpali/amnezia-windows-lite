/* SPDX-License-Identifier: MIT
 * Copyright (C) 2019-2022 WireGuard LLC. All Rights Reserved.
 */
package ui

import (
	"fmt"
	"github.com/lxn/walk"
	"github.com/mtpali/amnezia-windows-lite/manager"
	"time"
)

// Compact connection details; keys and protocol diagnostics stay in the editor.
type ConfView struct {
	*walk.Composite
	title, status, endpoint, traffic, handshake *walk.Label
	connect                                     *walk.PushButton
	tunnel                                      *manager.Tunnel
	busy                                        bool
}

func NewConfView(parent walk.Container) (*ConfView, error) {
	base, err := walk.NewComposite(parent)
	if err != nil {
		return nil, err
	}
	cv := &ConfView{Composite: base}
	layout := walk.NewVBoxLayout()
	layout.SetMargins(walk.Margins{10, 4, 0, 0})
	layout.SetSpacing(9)
	cv.SetLayout(layout)
	for _, field := range []**walk.Label{&cv.title, &cv.status, &cv.endpoint, &cv.traffic, &cv.handshake} {
		*field, err = walk.NewLabel(cv)
		if err != nil {
			base.Dispose()
			return nil, err
		}
		(*field).SetTextColor(oledText)
		(*field).SetMinMaxSize(walk.Size{0, 18}, walk.Size{0, 18})
	}
	font, err := walk.NewFont("Segoe UI", 13, walk.FontBold)
	if err != nil {
		base.Dispose()
		return nil, err
	}
	cv.AddDisposable(font)
	cv.title.SetFont(font)
	cv.title.SetMinMaxSize(walk.Size{0, 24}, walk.Size{0, 24})
	cv.status.SetTextColor(oledMuted)
	cv.endpoint.SetTextColor(oledMuted)
	cv.traffic.SetTextColor(oledMuted)
	cv.handshake.SetTextColor(oledMuted)
	walk.NewVSpacer(cv)
	cv.connect, err = walk.NewPushButton(cv)
	if err != nil {
		base.Dispose()
		return nil, err
	}
	cv.connect.SetMinMaxSize(walk.Size{0, 36}, walk.Size{0, 36})
	cv.connect.SetText("Connect")
	cv.connect.SetEnabled(false)
	cv.connect.Clicked().Attach(func() {
		if cv.tunnel == nil {
			return
		}
		t := *cv.tunnel
		cv.connect.SetEnabled(false)
		go func() {
			_, err := t.Toggle()
			cv.Synchronize(func() {
				if err != nil {
					showErrorCustom(cv.Form(), "Connection", err.Error())
				}
				cv.refresh()
			})
		}()
	})
	changed := manager.IPCClientRegisterTunnelChange(func(t *manager.Tunnel, state, global manager.TunnelState, err error) {
		cv.Synchronize(cv.refresh)
	})
	ticker := time.NewTicker(2 * time.Second)
	done := make(chan struct{})
	cv.Disposing().Attach(func() { changed.Unregister(); ticker.Stop(); close(done) })
	go func() {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				cv.Synchronize(func() {
					if form := cv.Form(); form != nil && form.Visible() {
						cv.refresh()
					}
				})
			}
		}
	}()
	return cv, nil
}
func (cv *ConfView) SetTunnel(t *manager.Tunnel) {
	if t == nil {
		cv.tunnel = nil
		cv.title.SetText("")
		cv.connect.SetEnabled(false)
		return
	}
	if cv.tunnel == nil || cv.tunnel.Name != t.Name {
		copy := *t
		cv.tunnel = &copy
		cv.title.SetText(t.Name)
		cv.status.SetText("")
		cv.endpoint.SetText("")
		cv.traffic.SetText("")
		cv.handshake.SetText("")
	}
	cv.refresh()
}
func (cv *ConfView) refresh() {
	if cv.tunnel == nil || cv.busy {
		return
	}
	cv.busy = true
	t := *cv.tunnel
	go func() {
		state, err := t.State()
		global, _ := manager.IPCClientGlobalState()
		cfg, cfgErr := t.StoredConfig()
		if state == manager.TunnelStarted {
			cfg, cfgErr = t.RuntimeConfig()
		}
		cv.Synchronize(func() {
			cv.busy = false
			if cv.tunnel == nil || cv.tunnel.Name != t.Name {
				cv.refresh()
				return
			}
			if err != nil {
				return
			}
			cv.status.SetText(textForState(state, false))
			cv.status.SetTextColor(oledMuted)
			cv.connect.SetText("Connect")
			if state == manager.TunnelStarted {
				cv.connect.SetText("Disconnect")
				cv.status.SetTextColor(oledAccent)
			}
			if state == manager.TunnelStarting || state == manager.TunnelStopping {
				cv.connect.SetText(textForState(state, true))
			}
			cv.connect.SetEnabled(global == manager.TunnelStarted || global == manager.TunnelStopped)
			if cfgErr == nil && len(cfg.Peers) > 0 {
				cv.endpoint.SetText(cfg.Peers[0].Endpoint.String())
				var rx, tx uint64
				for _, p := range cfg.Peers {
					rx += uint64(p.RxBytes)
					tx += uint64(p.TxBytes)
				}
				cv.traffic.SetText(fmt.Sprintf("↓ %s   ↑ %s", formatBytes(rx), formatBytes(tx)))
				hs := cfg.Peers[0].LastHandshakeTime
				if !hs.IsEmpty() {
					cv.handshake.SetText(hs.String())
				} else {
					cv.handshake.SetText("")
				}
			}
		})
	}()
}
func formatBytes(n uint64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	value := float64(n) / 1024
	for _, unit := range []string{"KB", "MB", "GB", "TB"} {
		if value < 1024 || unit == "TB" {
			return fmt.Sprintf("%.1f %s", value, unit)
		}
		value /= 1024
	}
	return ""
}

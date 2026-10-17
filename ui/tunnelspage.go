/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2019-2022 WireGuard LLC. All Rights Reserved.
 */

package ui

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lxn/walk"

	"github.com/amnezia-vpn/amneziawg-windows/v3/conf"
	"github.com/mtpali/amnezia-windows-lite/l18n"
	"github.com/mtpali/amnezia-windows-lite/manager"
)

type TunnelsPage struct {
	*walk.Composite

	listView      *ListView
	listContainer walk.Container
	confView      *ConfView
	fillerButton  *walk.PushButton
	fillerHandler func()

	fillerContainer        *walk.Composite
	currentTunnelContainer *walk.Composite
}

func NewTunnelsPage(parent walk.Container) (*TunnelsPage, error) {
	base, err := walk.NewComposite(parent)
	if err != nil {
		return nil, err
	}
	tp := &TunnelsPage{Composite: base}
	layout := walk.NewVBoxLayout()
	layout.SetMargins(walk.Margins{})
	layout.SetSpacing(8)
	tp.SetLayout(layout)
	body, err := walk.NewComposite(tp)
	if err != nil {
		return nil, err
	}
	row := walk.NewHBoxLayout()
	row.SetMargins(walk.Margins{})
	row.SetSpacing(6)
	body.SetLayout(row)
	list, err := walk.NewComposite(body)
	if err != nil {
		return nil, err
	}
	tp.listContainer = list
	list.SetMinMaxSize(walk.Size{150, 0}, walk.Size{150, 0})
	column := walk.NewVBoxLayout()
	column.SetMargins(walk.Margins{})
	column.SetSpacing(0)
	list.SetLayout(column)
	tp.listView, err = NewListView(list)
	if err != nil {
		return nil, err
	}
	tp.currentTunnelContainer, err = walk.NewComposite(body)
	if err != nil {
		return nil, err
	}
	details := walk.NewVBoxLayout()
	details.SetMargins(walk.Margins{})
	details.SetSpacing(0)
	tp.currentTunnelContainer.SetLayout(details)
	row.SetStretchFactor(tp.currentTunnelContainer, 100)
	tp.confView, err = NewConfView(tp.currentTunnelContainer)
	if err != nil {
		return nil, err
	}
	tp.fillerContainer, err = walk.NewComposite(body)
	if err != nil {
		return nil, err
	}
	filler := walk.NewVBoxLayout()
	filler.SetMargins(walk.Margins{})
	filler.SetAlignment(walk.AlignHCenterVCenter)
	tp.fillerContainer.SetLayout(filler)
	tp.fillerContainer.SetVisible(false)
	row.SetStretchFactor(tp.fillerContainer, 100)
	tp.fillerButton, err = walk.NewPushButton(tp.fillerContainer)
	if err != nil {
		return nil, err
	}
	tp.fillerButton.SetVisible(IsAdmin)
	tp.fillerButton.Clicked().Attach(func() {
		if tp.fillerHandler != nil {
			tp.fillerHandler()
		}
	})
	tp.listView.ItemCountChanged().Attach(tp.onTunnelsChanged)
	tp.listView.SelectedIndexesChanged().Attach(tp.onSelectedTunnelsChanged)
	tp.listView.ItemActivated().Attach(tp.onTunnelsViewItemActivated)
	tp.listView.CurrentIndexChanged().Attach(tp.updateConfView)
	tp.listView.Load(false)
	tp.onTunnelsChanged()
	return tp, nil
}

func (tp *TunnelsPage) CreateToolbar() error {
	bar, err := walk.NewComposite(tp)
	if err != nil {
		return err
	}
	layout := walk.NewHBoxLayout()
	layout.SetMargins(walk.Margins{})
	layout.SetSpacing(6)
	bar.SetLayout(layout)
	for _, item := range []struct {
		label string
		run   func()
		admin bool
	}{
		{"Import", tp.onImport, true}, {"Edit", tp.onEditTunnel, true}, {"Export", tp.onExportTunnels, true},
	} {
		button, err := walk.NewPushButton(bar)
		if err != nil {
			return err
		}
		button.SetText(item.label)
		button.SetMinMaxSize(walk.Size{64, 28}, walk.Size{0, 28})
		button.SetVisible(!item.admin || IsAdmin)
		button.Clicked().Attach(item.run)
		if item.label == "Edit" || item.label == "Export" {
			label := item.label
			update := func() {
				if label == "Edit" {
					button.SetEnabled(tp.listView.CurrentTunnel() != nil)
				} else {
					button.SetEnabled(tp.listView.model.RowCount() > 0)
				}
			}
			tp.listView.CurrentIndexChanged().Attach(update)
			tp.listView.ItemCountChanged().Attach(update)
			update()
		}
	}
	menu, err := walk.NewMenu()
	if err != nil {
		return err
	}
	tp.listView.AddDisposable(menu)
	for _, item := range []struct {
		label    string
		run      func()
		shortcut walk.Shortcut
		admin    bool
	}{
		{"Connect / disconnect", tp.onTunnelsViewItemActivated, walk.Shortcut{}, false},
		{"Import", tp.onImport, walk.Shortcut{walk.ModControl, walk.KeyO}, true},
		{"New", tp.onAddTunnel, walk.Shortcut{walk.ModControl, walk.KeyN}, true},
		{"Edit", tp.onEditTunnel, walk.Shortcut{walk.ModControl, walk.KeyE}, true},
		{"Delete", tp.onDelete, walk.Shortcut{0, walk.KeyDelete}, true},
		{"Select all", tp.onSelectAll, walk.Shortcut{walk.ModControl, walk.KeyA}, true},
	} {
		action := walk.NewAction()
		action.SetText(item.label)
		action.SetVisible(!item.admin || IsAdmin)
		action.Triggered().Attach(item.run)
		action.SetShortcut(item.shortcut)
		menu.Actions().Add(action)
		if item.shortcut.Key != 0 {
			tp.ShortcutActions().Add(action)
		}
	}
	tp.listView.SetContextMenu(menu)
	return nil
}

func (tp *TunnelsPage) updateConfView() {
	tp.confView.SetTunnel(tp.listView.CurrentTunnel())
}

func (tp *TunnelsPage) importFiles(paths []string) {
	go func() {
		syncedMsgBox := func(title, message string, flags walk.MsgBoxStyle) {
			tp.Synchronize(func() {
				oledMessageBox(tp.Form(), title, message, flags)
			})
		}
		type unparsedConfig struct {
			Name   string
			Config string
		}

		var (
			unparsedConfigs []unparsedConfig
			lastErr         error
		)

		for _, path := range paths {
			switch strings.ToLower(filepath.Ext(path)) {
			case ".conf":
				textConfig, err := os.ReadFile(path)
				if err != nil {
					lastErr = err
					continue
				}
				unparsedConfigs = append(unparsedConfigs, unparsedConfig{Name: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), Config: string(textConfig)})
			case ".zip":
				// 1 .conf + 1 error .zip edge case?
				r, err := zip.OpenReader(path)
				if err != nil {
					lastErr = err
					continue
				}

				for _, f := range r.File {
					if strings.ToLower(filepath.Ext(f.Name)) != ".conf" {
						continue
					}

					rc, err := f.Open()
					if err != nil {
						lastErr = err
						continue
					}
					textConfig, err := io.ReadAll(rc)
					rc.Close()
					if err != nil {
						lastErr = err
						continue
					}
					unparsedConfigs = append(unparsedConfigs, unparsedConfig{Name: strings.TrimSuffix(filepath.Base(f.Name), filepath.Ext(f.Name)), Config: string(textConfig)})
				}

				r.Close()
			}
		}

		if lastErr != nil || unparsedConfigs == nil {
			if lastErr == nil {
				lastErr = errors.New(l18n.Sprintf("no configuration files were found"))
			}
			syncedMsgBox(l18n.Sprintf("Error"), l18n.Sprintf("Could not import selected configuration: %v", lastErr), walk.MsgBoxIconWarning)
			return
		}

		// Import deterministically; selection is restored independently of file order.
		sort.Slice(unparsedConfigs, func(i, j int) bool {
			return conf.TunnelNameIsLess(unparsedConfigs[j].Name, unparsedConfigs[i].Name)
		})

		existingTunnelList, err := manager.IPCClientTunnels()
		if err != nil {
			syncedMsgBox(l18n.Sprintf("Error"), l18n.Sprintf("Could not enumerate existing tunnels: %v", lastErr), walk.MsgBoxIconWarning)
			return
		}
		existingLowerTunnels := make(map[string]bool, len(existingTunnelList))
		for _, tunnel := range existingTunnelList {
			existingLowerTunnels[strings.ToLower(tunnel.Name)] = true
		}

		configCount := 0
		tp.listView.SetSuspendTunnelsUpdate(true)
		for _, unparsedConfig := range unparsedConfigs {
			if existingLowerTunnels[strings.ToLower(unparsedConfig.Name)] {
				lastErr = errors.New(l18n.Sprintf("Another tunnel already exists with the name ‘%s’", unparsedConfig.Name))
				continue
			}
			config, err := conf.FromWgQuickWithUnknownEncoding(unparsedConfig.Config, unparsedConfig.Name)
			if err != nil {
				lastErr = err
				continue
			}
			_, err = manager.IPCClientNewTunnel(config)
			if err != nil {
				lastErr = err
				continue
			}
			configCount++
		}
		tp.listView.SetSuspendTunnelsUpdate(false)

		m, n := configCount, len(unparsedConfigs)
		switch {
		case n == 1 && m != n:
			syncedMsgBox(l18n.Sprintf("Error"), l18n.Sprintf("Unable to import configuration: %v", lastErr), walk.MsgBoxIconWarning)
		case n == 1 && m == n:
			// nothing
		case m == n:
			// Successful imports need no additional dialog.
		case m != n:
			syncedMsgBox(l18n.Sprintf("Imported tunnels"), l18n.Sprintf("Imported %d of %d tunnels", m, n), walk.MsgBoxIconWarning)
		}
	}()
}

func (tp *TunnelsPage) exportTunnels(filePath string) {
	writeFileWithOverwriteHandling(tp.Form(), filePath, func(file *os.File) error {
		writer := zip.NewWriter(file)

		for _, tunnel := range tp.listView.model.tunnels {
			cfg, err := tunnel.StoredConfig()
			if err != nil {
				return fmt.Errorf("onExportTunnels: tunnel.StoredConfig failed: %w", err)
			}

			w, err := writer.Create(tunnel.Name + ".conf")
			if err != nil {
				return fmt.Errorf("onExportTunnels: writer.Create failed: %w", err)
			}

			if _, err := w.Write(([]byte)(cfg.ToWgQuick())); err != nil {
				return fmt.Errorf("onExportTunnels: cfg.ToWgQuick failed: %w", err)
			}
		}

		return writer.Close()
	})
}

func (tp *TunnelsPage) addTunnel(config *conf.Config) {
	_, err := manager.IPCClientNewTunnel(config)
	if err != nil {
		showErrorCustom(tp.Form(), l18n.Sprintf("Unable to create tunnel"), err.Error())
	}
}

// Handlers

func (tp *TunnelsPage) onTunnelsViewItemActivated() {
	t := tp.listView.CurrentTunnel()
	if t == nil {
		return
	}
	tunnel := *t
	go func() {
		globalState, err := manager.IPCClientGlobalState()
		if err != nil || (globalState != manager.TunnelStarted && globalState != manager.TunnelStopped) {
			return
		}
		oldState, err := tunnel.Toggle()
		if err != nil {
			tp.Synchronize(func() {
				if oldState == manager.TunnelUnknown {
					showErrorCustom(tp.Form(), l18n.Sprintf("Failed to determine tunnel state"), err.Error())
				} else if oldState == manager.TunnelStopped {
					showErrorCustom(tp.Form(), l18n.Sprintf("Failed to activate tunnel"), err.Error())
				} else if oldState == manager.TunnelStarted {
					showErrorCustom(tp.Form(), l18n.Sprintf("Failed to deactivate tunnel"), err.Error())
				}
			})
			return
		}
	}()
}

func (tp *TunnelsPage) onEditTunnel() {
	tunnel := tp.listView.CurrentTunnel()
	if tunnel == nil {
		return
	}

	if config := runEditDialog(tp.Form(), tunnel); config != nil {
		go func() {
			priorState, err := tunnel.State()
			if deleteErr := tunnel.Delete(); deleteErr != nil {
				tp.Synchronize(func() { showError(deleteErr, tp.Form()) })
				return
			}
			if stopErr := tunnel.WaitForStop(); stopErr != nil {
				tp.Synchronize(func() { showError(stopErr, tp.Form()) })
				return
			}
			tunnel, err2 := manager.IPCClientNewTunnel(config)
			if err == nil && err2 == nil && (priorState == manager.TunnelStarting || priorState == manager.TunnelStarted) {
				tunnel.Start()
			}
		}()
	}
}

func (tp *TunnelsPage) onAddTunnel() {
	if config := runEditDialog(tp.Form(), nil); config != nil {
		// Save new
		tp.addTunnel(config)
	}
}

func (tp *TunnelsPage) onDelete() {
	indices := tp.listView.SelectedIndexes()
	if len(indices) == 0 {
		return
	}

	var title, question string
	if len(indices) > 1 {
		tunnelCount := len(indices)
		title = l18n.Sprintf("Delete %d", tunnelCount)
		question = l18n.Sprintf("Are you sure you would like to delete %d tunnels?", tunnelCount)
	} else {
		tunnelName := tp.listView.model.tunnels[indices[0]].Name
		title = l18n.Sprintf("Delete tunnel ‘%s’", tunnelName)
		question = l18n.Sprintf("Are you sure you would like to delete tunnel ‘%s’?", tunnelName)
	}
	if walk.DlgCmdYes != oledMessageBox(
		tp.Form(),
		title,
		l18n.Sprintf("%s You cannot undo this action.", question),
		walk.MsgBoxYesNo|walk.MsgBoxIconWarning) {
		return
	}

	selectTunnelAfter := ""
	if len(indices) < len(tp.listView.model.tunnels) {
		sort.Ints(indices)
		max := 0
		for i, idx := range indices {
			if idx+1 < len(tp.listView.model.tunnels) && (i+1 == len(indices) || idx+1 != indices[i+1]) {
				max = idx + 1
			} else if idx-1 >= 0 && (i == 0 || idx-1 != indices[i-1]) {
				max = idx - 1
			}
		}
		selectTunnelAfter = tp.listView.model.tunnels[max].Name
	}
	if len(selectTunnelAfter) > 0 {
		tp.listView.selectTunnel(selectTunnelAfter)
	}

	tunnelsToDelete := make([]manager.Tunnel, len(indices))
	for i, j := range indices {
		tunnelsToDelete[i] = tp.listView.model.tunnels[j]
	}
	go func() {
		tp.listView.SetSuspendTunnelsUpdate(true)
		var errors []error
		for _, tunnel := range tunnelsToDelete {
			err := tunnel.Delete()
			if err != nil && (len(errors) == 0 || errors[len(errors)-1].Error() != err.Error()) {
				errors = append(errors, err)
			}
		}
		tp.listView.SetSuspendTunnelsUpdate(false)
		if len(errors) > 0 {
			tp.listView.Synchronize(func() {
				if len(errors) == 1 {
					showErrorCustom(tp.Form(), l18n.Sprintf("Unable to delete tunnel"), l18n.Sprintf("A tunnel was unable to be removed: %s", errors[0].Error()))
				} else {
					showErrorCustom(tp.Form(), l18n.Sprintf("Unable to delete tunnels"), l18n.Sprintf("%d tunnels were unable to be removed.", len(errors)))
				}
			})
		}
	}()
}

func (tp *TunnelsPage) onSelectAll() {
	tp.listView.SetSelectedIndexes([]int{-1})
}

func (tp *TunnelsPage) onImport() {
	dlg := walk.FileDialog{
		Filter: l18n.Sprintf("Configuration Files (*.zip, *.conf)|*.zip;*.conf|All Files (*.*)|*.*"),
		Title:  "Import",
	}

	if ok, _ := dlg.ShowOpenMultiple(tp.Form()); !ok {
		return
	}

	tp.importFiles(dlg.FilePaths)
}

func (tp *TunnelsPage) onExportTunnels() {
	dlg := walk.FileDialog{
		Filter: l18n.Sprintf("Configuration ZIP Files (*.zip)|*.zip"),
		Title:  l18n.Sprintf("Export tunnels to zip"),
	}

	if ok, _ := dlg.ShowSave(tp.Form()); !ok {
		return
	}

	if !strings.HasSuffix(dlg.FilePath, ".zip") {
		dlg.FilePath += ".zip"
	}

	tp.exportTunnels(dlg.FilePath)
}

func (tp *TunnelsPage) swapFiller(enabled bool) bool {
	if tp.fillerContainer.Visible() == enabled {
		return enabled
	}
	tp.SetSuspended(true)
	tp.fillerContainer.SetVisible(enabled)
	tp.currentTunnelContainer.SetVisible(!enabled)
	tp.SetSuspended(false)
	return enabled
}

func (tp *TunnelsPage) onTunnelsChanged() {
	if tp.swapFiller(tp.listView.model.RowCount() == 0) {
		tp.fillerButton.SetText("Import")
		tp.fillerHandler = tp.onImport
	}
}

func (tp *TunnelsPage) onSelectedTunnelsChanged() {
	if tp.listView.model.RowCount() == 0 {
		return
	}
	indices := tp.listView.SelectedIndexes()
	tunnelCount := len(indices)
	if tp.swapFiller(tunnelCount > 1) {
		tp.fillerButton.SetText(l18n.Sprintf("Delete %d", tunnelCount))
		tp.fillerHandler = tp.onDelete
	}
}

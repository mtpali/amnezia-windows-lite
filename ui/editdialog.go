/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2019-2022 WireGuard LLC. All Rights Reserved.
 */

package ui

import (
	"strings"

	"github.com/lxn/walk"
	"github.com/lxn/win"
	"golang.org/x/sys/windows"

	"github.com/amnezia-vpn/amneziawg-windows/v3/conf"
	"github.com/mtpali/amnezia-windows-lite/l18n"
	"github.com/mtpali/amnezia-windows-lite/manager"
	"github.com/mtpali/amnezia-windows-lite/ui/syntax"
)

type EditDialog struct {
	*walk.Dialog
	nameEdit                        *walk.LineEdit
	pubkeyEdit                      *walk.LineEdit
	syntaxEdit                      *syntax.SyntaxEdit
	blockUntunneledTrafficCB        *walk.CheckBox
	saveButton                      *walk.PushButton
	config                          conf.Config
	lastPrivateKey                  string
	blockUntunneledTraficCheckGuard bool
}

func runEditDialog(owner walk.Form, tunnel *manager.Tunnel) *conf.Config {
	dlg, err := newEditDialog(owner, tunnel)
	if showError(err, owner) {
		return nil
	}

	if dlg.Run() == walk.DlgCmdOK {
		return &dlg.config
	}

	return nil
}

func newEditDialog(owner walk.Form, tunnel *manager.Tunnel) (*EditDialog, error) {
	var err error
	var disposables walk.Disposables
	defer disposables.Treat()

	dlg := new(EditDialog)

	var title string
	if tunnel == nil {
		title = l18n.Sprintf("Create new tunnel")
	} else {
		title = l18n.Sprintf("Edit tunnel")
	}

	if tunnel == nil {
		// Creating a new tunnel, create a new private key and use the default template
		pk, _ := conf.NewPrivateKey()
		dlg.config = conf.Config{Interface: conf.Interface{PrivateKey: *pk}}
	} else {
		dlg.config, _ = tunnel.StoredConfig()
	}

	layout := walk.NewGridLayout()
	layout.SetSpacing(6)
	layout.SetMargins(walk.Margins{10, 10, 10, 10})
	layout.SetColumnStretchFactor(1, 3)

	if dlg.Dialog, err = walk.NewDialog(owner); err != nil {
		return nil, err
	}
	disposables.Add(dlg)
	dlg.SetIcon(owner.Icon())
	dlg.SetTitle(title)
	dlg.SetLayout(layout)
	dlg.SetMinMaxSize(walk.Size{460, 360}, walk.Size{0, 0})
	if icon, err := loadSystemIcon("imageres", -114, 32); err == nil {
		dlg.SetIcon(icon)
	}

	nameLabel, err := walk.NewTextLabel(dlg)
	if err != nil {
		return nil, err
	}
	layout.SetRange(nameLabel, walk.Rectangle{0, 0, 1, 1})
	nameLabel.SetTextAlignment(walk.AlignHFarVCenter)
	nameLabel.SetText(l18n.Sprintf("&Name:"))

	if dlg.nameEdit, err = walk.NewLineEdit(dlg); err != nil {
		return nil, err
	}
	layout.SetRange(dlg.nameEdit, walk.Rectangle{1, 0, 1, 1})
	dlg.nameEdit.SetText(dlg.config.Name)

	pubkeyLabel, err := walk.NewTextLabel(dlg)
	if err != nil {
		return nil, err
	}
	layout.SetRange(pubkeyLabel, walk.Rectangle{0, 1, 1, 1})
	pubkeyLabel.SetTextAlignment(walk.AlignHFarVCenter)
	pubkeyLabel.SetText(l18n.Sprintf("&Public key:"))

	if dlg.pubkeyEdit, err = walk.NewLineEdit(dlg); err != nil {
		return nil, err
	}
	layout.SetRange(dlg.pubkeyEdit, walk.Rectangle{1, 1, 1, 1})
	dlg.pubkeyEdit.SetReadOnly(true)
	dlg.pubkeyEdit.SetText(l18n.Sprintf("(unknown)"))
	dlg.pubkeyEdit.Accessibility().SetRole(walk.AccRoleStatictext)

	if dlg.syntaxEdit, err = syntax.NewSyntaxEdit(dlg); err != nil {
		return nil, err
	}
	layout.SetRange(dlg.syntaxEdit, walk.Rectangle{0, 2, 2, 1})

	buttonsContainer, err := walk.NewComposite(dlg)
	if err != nil {
		return nil, err
	}
	layout.SetRange(buttonsContainer, walk.Rectangle{0, 3, 2, 1})
	buttonsContainer.SetLayout(walk.NewHBoxLayout())
	buttonsContainer.Layout().SetMargins(walk.Margins{})

	if dlg.blockUntunneledTrafficCB, err = walk.NewCheckBox(buttonsContainer); err != nil {
		return nil, err
	}
	dlg.blockUntunneledTrafficCB.SetText("Block untunneled traffic")
	dlg.blockUntunneledTrafficCB.SetChecked(dlg.config.Interface.BlockUntunneledTraffic)
	dlg.blockUntunneledTrafficCB.CheckedChanged().Attach(dlg.onBlockChanged)

	walk.NewHSpacer(buttonsContainer)

	if dlg.saveButton, err = walk.NewPushButton(buttonsContainer); err != nil {
		return nil, err
	}
	dlg.saveButton.SetText(l18n.Sprintf("&Save"))
	dlg.saveButton.Clicked().Attach(dlg.onSaveButtonClicked)

	cancelButton, err := walk.NewPushButton(buttonsContainer)
	if err != nil {
		return nil, err
	}
	cancelButton.SetText(l18n.Sprintf("Cancel"))
	cancelButton.Clicked().Attach(dlg.Cancel)

	dlg.SetCancelButton(cancelButton)
	dlg.SetDefaultButton(dlg.saveButton)

	dlg.syntaxEdit.TextChanged().Attach(dlg.onSyntaxEditTextChanged)
	dlg.syntaxEdit.PrivateKeyChanged().Attach(dlg.onSyntaxEditPrivateKeyChanged)
	dlg.syntaxEdit.SetText(dlg.config.ToWgQuick())

	// Insert a dummy label immediately preceding syntaxEdit to have screen readers read it.
	// Otherwise they fallback to "RichEdit Control".
	syntaxEditWnd := dlg.syntaxEdit.Handle()
	parentWnd := win.GetParent(syntaxEditWnd)
	labelWnd := win.CreateWindowEx(0,
		windows.StringToUTF16Ptr("STATIC"), windows.StringToUTF16Ptr(l18n.Sprintf("&Configuration:")),
		win.WS_CHILD|win.WS_GROUP|win.SS_LEFT, 0, 0, 0, 0,
		parentWnd, win.HMENU(^uintptr(0)), win.HINSTANCE(win.GetWindowLongPtr(parentWnd, win.GWLP_HINSTANCE)), nil)
	prevWnd := win.GetWindow(syntaxEditWnd, win.GW_HWNDPREV)
	nextWnd := win.GetWindow(syntaxEditWnd, win.GW_HWNDNEXT)
	win.SetWindowPos(labelWnd, prevWnd, 0, 0, 0, 0, win.SWP_NOSIZE|win.SWP_NOMOVE)
	win.SetWindowPos(syntaxEditWnd, labelWnd, 0, 0, 0, 0, win.SWP_NOSIZE|win.SWP_NOMOVE)
	win.SetWindowPos(nextWnd, syntaxEditWnd, 0, 0, 0, 0, win.SWP_NOSIZE|win.SWP_NOMOVE)

	if tunnel != nil {
		dlg.Starting().Attach(func() {
			dlg.syntaxEdit.SetFocus()
		})
	}

	applyOLED(dlg)
	disposables.Spare()

	return dlg, nil
}

func (dlg *EditDialog) onSyntaxEditPrivateKeyChanged(privateKey string) {
	if privateKey == dlg.lastPrivateKey {
		return
	}
	dlg.lastPrivateKey = privateKey
	key, _ := conf.NewPrivateKeyFromString(privateKey)
	if key != nil {
		dlg.pubkeyEdit.SetText(key.Public().String())
	} else {
		dlg.pubkeyEdit.SetText(l18n.Sprintf("(unknown)"))
	}
}

func (dlg *EditDialog) onBlockChanged() {
	if dlg.blockUntunneledTraficCheckGuard {
		return
	}
	cfg, err := conf.FromWgQuick(dlg.syntaxEdit.Text(), "temporary")
	if err != nil {
		return
	}
	cfg.Interface.BlockUntunneledTraffic = dlg.blockUntunneledTrafficCB.Checked()
	dlg.syntaxEdit.SetText(cfg.ToWgQuick())
}

func (dlg *EditDialog) onSyntaxEditTextChanged() {
	if cfg, err := conf.FromWgQuick(dlg.syntaxEdit.Text(), "temporary"); err == nil {
		dlg.blockUntunneledTraficCheckGuard = true
		dlg.blockUntunneledTrafficCB.SetChecked(cfg.Interface.BlockUntunneledTraffic)
		dlg.blockUntunneledTraficCheckGuard = false
	}
	dlg.saveButton.SetEnabled(!dlg.syntaxEdit.HaveErrors())
}

func (dlg *EditDialog) onSaveButtonClicked() {
	newName := dlg.nameEdit.Text()
	if newName == "" {
		showWarningCustom(dlg, l18n.Sprintf("Invalid name"), l18n.Sprintf("A name is required."))
		return
	}
	if !conf.TunnelNameIsValid(newName) {
		showWarningCustom(dlg, l18n.Sprintf("Invalid name"), l18n.Sprintf("Tunnel name ‘%s’ is invalid.", newName))
		return
	}
	newNameLower := strings.ToLower(newName)

	if newNameLower != strings.ToLower(dlg.config.Name) {
		existingTunnelList, err := manager.IPCClientTunnels()
		if err != nil {
			showWarningCustom(dlg, l18n.Sprintf("Unable to list existing tunnels"), err.Error())
			return
		}
		for _, tunnel := range existingTunnelList {
			if strings.ToLower(tunnel.Name) == newNameLower {
				showWarningCustom(dlg, l18n.Sprintf("Tunnel already exists"), l18n.Sprintf("Another tunnel already exists with the name ‘%s’.", newName))
				return
			}
		}
	}

	cfg, err := conf.FromWgQuick(dlg.syntaxEdit.Text(), newName)
	if err != nil {
		showErrorCustom(dlg, l18n.Sprintf("Unable to create new configuration"), err.Error())
		return
	}

	cfg.Interface.BlockUntunneledTraffic = dlg.blockUntunneledTrafficCB.Checked()
	dlg.config = *cfg
	dlg.Accept()
}

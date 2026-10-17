// SPDX-License-Identifier: MIT
package ui

import "github.com/lxn/walk"

func oledMessageBox(owner walk.Form, title, message string, style walk.MsgBoxStyle) int {
	dlg, err := walk.NewDialog(owner)
	if err != nil {
		return walk.DlgCmdCancel
	}
	defer dlg.Dispose()
	dlg.SetTitle(title)
	dlg.SetLayout(walk.NewVBoxLayout())
	dlg.SetSize(walk.Size{430, 180})
	dlg.SetMinMaxSize(walk.Size{360, 150}, walk.Size{})
	label, err := walk.NewTextLabel(dlg)
	if err != nil {
		return walk.DlgCmdCancel
	}
	label.SetText(message)
	label.SetMinMaxSize(walk.Size{320, 0}, walk.Size{460, 0})
	buttons, _ := walk.NewComposite(dlg)
	buttons.SetLayout(walk.NewHBoxLayout())
	walk.NewHSpacer(buttons)
	if style&walk.MsgBoxYesNo == walk.MsgBoxYesNo {
		yes, _ := walk.NewPushButton(buttons)
		yes.SetText("Yes")
		yes.Clicked().Attach(func() { dlg.Close(walk.DlgCmdYes) })
		no, _ := walk.NewPushButton(buttons)
		no.SetText("No")
		no.Clicked().Attach(func() { dlg.Close(walk.DlgCmdNo) })
		dlg.SetDefaultButton(no)
		dlg.SetCancelButton(no)
	} else {
		ok, _ := walk.NewPushButton(buttons)
		ok.SetText("OK")
		ok.Clicked().Attach(dlg.Accept)
		dlg.SetDefaultButton(ok)
		dlg.SetCancelButton(ok)
	}
	applyOLED(dlg)
	result := dlg.Run()
	if result == walk.DlgCmdCancel && style&walk.MsgBoxYesNo == walk.MsgBoxYesNo {
		return walk.DlgCmdNo
	}
	return result
}

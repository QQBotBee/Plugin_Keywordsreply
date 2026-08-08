//go:build windows

package main

import (
	"errors"
	"os"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

const (
	settingsBaseWidth   = 980
	settingsBaseHeight  = 640
	settingsBasePadding = 20
)

type settingsLayout struct {
	ClientWidth  int
	ClientHeight int
	Padding      int
	Resizable    bool
	Maximizable  bool
}

func scaleDPI(value, dpi int) int {
	if dpi <= 0 {
		dpi = 96
	}
	return (value*dpi + 48) / 96
}

func defaultSettingsLayout(dpi int) settingsLayout {
	return settingsLayout{
		ClientWidth:  scaleDPI(settingsBaseWidth, dpi),
		ClientHeight: scaleDPI(settingsBaseHeight, dpi),
		Padding:      scaleDPI(settingsBasePadding, dpi),
		Resizable:    false,
		Maximizable:  false,
	}
}

func centeredPosition(screenWidth, screenHeight, windowWidth, windowHeight int) (int, int) {
	x := (screenWidth - windowWidth) / 2
	y := (screenHeight - windowHeight) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	return x, y
}

const (
	idRuleList = 1001 + iota
	idKeyword
	idMatchMode
	idCaseSensitive
	idAreaFriend
	idAreaGroup
	idAreaChannel
	idAreaChannelPrivate
	idReplyType
	idReplyContent
	idAddRule
	idSaveRule
	idDeleteRule
	idMoveUp
	idMoveDown
	idStatus
)

const (
	synchronize         = 0x00100000
	wmCreate            = 0x0001
	wmDestroy           = 0x0002
	wmClose             = 0x0010
	wmSetFont           = 0x0030
	wmCommand           = 0x0111
	swShow              = 5
	swRestore           = 9
	wsExTopmost         = 0x00000008
	wsOverlapped        = 0x00000000
	wsCaption           = 0x00c00000
	wsSysMenu           = 0x00080000
	wsMinimizeBox       = 0x00020000
	wsChild             = 0x40000000
	wsVisible           = 0x10000000
	wsTabStop           = 0x00010000
	wsBorder            = 0x00800000
	wsVScroll           = 0x00200000
	esAutoHScroll       = 0x0080
	esMultiline         = 0x0004
	esAutoVScroll       = 0x0040
	esWantReturn        = 0x1000
	bsPushButton        = 0x0000
	bsAutoCheckbox      = 0x0003
	lbsNotify           = 0x0001
	lbsNoIntegralHeight = 0x0100
	cbsDropdownList     = 0x0003
	colorWindow         = 5
	idiApplication      = 32512
	idcArrow            = 32512
	logPixelsX          = 88
	smCxScreen          = 0
	smCyScreen          = 1
	swpNoSize           = 0x0001
	swpNoMove           = 0x0002
	swpShowWindow       = 0x0040
	mbOK                = 0x0000
	mbIconError         = 0x0010
	defaultGUIFont      = 17
	lbAddString         = 0x0180
	lbResetContent      = 0x0184
	lbSetCurSel         = 0x0186
	lbGetCurSel         = 0x0188
	cbAddString         = 0x0143
	cbGetCurSel         = 0x0147
	cbSetCurSel         = 0x014e
	bmGetCheck          = 0x00f0
	bmSetCheck          = 0x00f1
	bstUnchecked        = 0
	bstChecked          = 1
	bnClicked           = 0
	lbnSelChange        = 1
	cbnSelChange        = 1
)

var hwndTopmost = ^uintptr(0)

var (
	kernel32                = syscall.NewLazyDLL("kernel32.dll")
	user32                  = syscall.NewLazyDLL("user32.dll")
	gdi32                   = syscall.NewLazyDLL("gdi32.dll")
	procRegisterClassExW    = user32.NewProc("RegisterClassExW")
	procUnregisterClassW    = user32.NewProc("UnregisterClassW")
	procDestroyWindow       = user32.NewProc("DestroyWindow")
	procCreateWindowExW     = user32.NewProc("CreateWindowExW")
	procDefWindowProcW      = user32.NewProc("DefWindowProcW")
	procShowWindow          = user32.NewProc("ShowWindow")
	procUpdateWindow        = user32.NewProc("UpdateWindow")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procSetWindowPos        = user32.NewProc("SetWindowPos")
	procIsIconic            = user32.NewProc("IsIconic")
	procPostMessageW        = user32.NewProc("PostMessageW")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessageW    = user32.NewProc("DispatchMessageW")
	procIsDialogMessageW    = user32.NewProc("IsDialogMessageW")
	procPostQuitMessage     = user32.NewProc("PostQuitMessage")
	procAdjustWindowRectEx  = user32.NewProc("AdjustWindowRectEx")
	procLoadCursorW         = user32.NewProc("LoadCursorW")
	procLoadIconW           = user32.NewProc("LoadIconW")
	procGetSystemMetrics    = user32.NewProc("GetSystemMetrics")
	procGetDC               = user32.NewProc("GetDC")
	procReleaseDC           = user32.NewProc("ReleaseDC")
	procGetDeviceCaps       = gdi32.NewProc("GetDeviceCaps")
	procGetModuleHandleW    = kernel32.NewProc("GetModuleHandleW")
	procSendMessageW        = user32.NewProc("SendMessageW")
	procSetWindowTextW      = user32.NewProc("SetWindowTextW")
	procGetWindowTextW      = user32.NewProc("GetWindowTextW")
	procGetWindowTextLength = user32.NewProc("GetWindowTextLengthW")
	procEnableWindow        = user32.NewProc("EnableWindow")
	procMessageBoxW         = user32.NewProc("MessageBoxW")
	procGetStockObject      = gdi32.NewProc("GetStockObject")
	openProcess             = kernel32.NewProc("OpenProcess")
	waitSingleObject        = kernel32.NewProc("WaitForSingleObject")
	closeHandle             = kernel32.NewProc("CloseHandle")
)

type point struct{ X, Y int32 }
type rect struct{ Left, Top, Right, Bottom int32 }
type msg struct {
	HWnd, Message, WParam, LParam uintptr
	Time                          uint32
	Pt                            point
	Private                       uint32
}
type wndClassEx struct {
	Size, Style          uint32
	WndProc, ClsExtra    uintptr
	WndExtra, Instance   uintptr
	Icon, Cursor         uintptr
	Background, MenuName uintptr
	ClassName, IconSmall uintptr
}

type settingsUI struct {
	controller *SettingsController
	controls   map[int]uintptr
	selected   int
}

var settingsNative = struct {
	sync.Mutex
	hwnd       uintptr
	done       chan struct{}
	closing    bool
	controller *SettingsController
	ui         *settingsUI
}{}

var settingsWndProc = syscall.NewCallback(settingsWindowProc)

func showSettingsWindow() error {
	store := currentKeywordStore()
	if store == nil {
		err := errors.New("关键词规则尚未初始化，请重新启用插件后再打开设置")
		showSettingsError(0, err.Error())
		return err
	}

	settingsNative.Lock()
	if settingsNative.hwnd != 0 {
		hwnd := settingsNative.hwnd
		settingsNative.Unlock()
		iconic, _, _ := procIsIconic.Call(hwnd)
		if iconic != 0 {
			procShowWindow.Call(hwnd, swRestore)
		}
		procSetWindowPos.Call(hwnd, hwndTopmost, 0, 0, 0, 0, swpNoMove|swpNoSize|swpShowWindow)
		procSetForegroundWindow.Call(hwnd)
		return nil
	}
	if settingsNative.done != nil || settingsNative.closing {
		settingsNative.Unlock()
		return nil
	}
	settingsNative.done = make(chan struct{})
	settingsNative.controller = NewSettingsController(store)
	done := settingsNative.done
	settingsNative.Unlock()
	go settingsWindowThread(done)
	return nil
}

func closeSettingsWindow() {
	settingsNative.Lock()
	done, hwnd := settingsNative.done, settingsNative.hwnd
	if done == nil {
		settingsNative.Unlock()
		return
	}
	settingsNative.closing = true
	settingsNative.Unlock()
	if hwnd != 0 {
		procPostMessageW.Call(hwnd, wmClose, 0, 0)
	}
	<-done
}

func settingsWindowThread(done chan struct{}) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer func() {
		settingsNative.Lock()
		settingsNative.hwnd = 0
		settingsNative.done = nil
		settingsNative.closing = false
		settingsNative.controller = nil
		settingsNative.ui = nil
		close(done)
		settingsNative.Unlock()
	}()

	instance, _, _ := procGetModuleHandleW.Call(0)
	className := utf16Ptr("BeeKeywordReplySettingsWindow")
	title := utf16Ptr(PluginName + " 设置")
	cursor, _, _ := procLoadCursorW.Call(0, idcArrow)
	icon, _, _ := procLoadIconW.Call(0, idiApplication)
	class := wndClassEx{Size: uint32(unsafe.Sizeof(wndClassEx{})), WndProc: settingsWndProc, Instance: instance, Icon: icon, Cursor: cursor, Background: colorWindow + 1, ClassName: uintptr(unsafe.Pointer(className)), IconSmall: icon}
	atom, _, _ := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&class)))
	if atom == 0 {
		return
	}
	defer procUnregisterClassW.Call(uintptr(unsafe.Pointer(className)), instance)

	dpi := settingsDPI()
	layout := defaultSettingsLayout(dpi)
	style := uintptr(wsOverlapped | wsCaption | wsSysMenu | wsMinimizeBox)
	windowRect := rect{Right: int32(layout.ClientWidth), Bottom: int32(layout.ClientHeight)}
	procAdjustWindowRectEx.Call(uintptr(unsafe.Pointer(&windowRect)), style, 0, 0)
	width, height := int(windowRect.Right-windowRect.Left), int(windowRect.Bottom-windowRect.Top)
	screenW, _, _ := procGetSystemMetrics.Call(smCxScreen)
	screenH, _, _ := procGetSystemMetrics.Call(smCyScreen)
	x, y := centeredPosition(int(screenW), int(screenH), width, height)
	hwnd, _, _ := procCreateWindowExW.Call(wsExTopmost, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(title)), style, uintptr(x), uintptr(y), uintptr(width), uintptr(height), 0, 0, instance, 0)
	if hwnd == 0 {
		return
	}
	settingsNative.Lock()
	settingsNative.hwnd = hwnd
	closing := settingsNative.closing
	settingsNative.Unlock()
	if closing {
		procPostMessageW.Call(hwnd, wmClose, 0, 0)
	}
	procShowWindow.Call(hwnd, swShow)
	procSetWindowPos.Call(hwnd, hwndTopmost, 0, 0, 0, 0, swpNoMove|swpNoSize|swpShowWindow)
	procSetForegroundWindow.Call(hwnd)
	procUpdateWindow.Call(hwnd)

	var message msg
	for {
		result, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(result) <= 0 {
			break
		}
		handled, _, _ := procIsDialogMessageW.Call(hwnd, uintptr(unsafe.Pointer(&message)))
		if handled == 0 {
			procTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
			procDispatchMessageW.Call(uintptr(unsafe.Pointer(&message)))
		}
	}
}

func settingsDPI() int {
	dc, _, _ := procGetDC.Call(0)
	if dc == 0 {
		return 96
	}
	defer procReleaseDC.Call(0, dc)
	dpi, _, _ := procGetDeviceCaps.Call(dc, logPixelsX)
	if dpi == 0 {
		return 96
	}
	return int(dpi)
}

func settingsWindowProc(hwnd uintptr, message uint32, wparam, lparam uintptr) uintptr {
	switch message {
	case wmCreate:
		settingsNative.Lock()
		controller := settingsNative.controller
		settingsNative.Unlock()
		ui := createSettingsUI(hwnd, controller)
		settingsNative.Lock()
		settingsNative.ui = ui
		settingsNative.Unlock()
		return 0
	case wmCommand:
		settingsNative.Lock()
		ui := settingsNative.ui
		settingsNative.Unlock()
		if ui != nil {
			ui.handleCommand(int(wparam&0xffff), int((wparam>>16)&0xffff))
		}
		return 0
	case wmClose:
		procDestroyWindow.Call(hwnd)
		return 0
	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}
	result, _, _ := procDefWindowProcW.Call(hwnd, uintptr(message), wparam, lparam)
	return result
}

func createSettingsUI(hwnd uintptr, controller *SettingsController) *settingsUI {
	dpi := settingsDPI()
	s := func(value int) int { return scaleDPI(value, dpi) }
	ui := &settingsUI{controller: controller, controls: make(map[int]uintptr), selected: -1}
	font, _, _ := procGetStockObject.Call(defaultGUIFont)

	create := func(id int, className, text string, style uintptr, x, y, width, height int) uintptr {
		control, _, _ := procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(utf16Ptr(className))), uintptr(unsafe.Pointer(utf16Ptr(text))), wsChild|wsVisible|style, uintptr(s(x)), uintptr(s(y)), uintptr(s(width)), uintptr(s(height)), hwnd, uintptr(id), 0, 0)
		if control != 0 {
			procSendMessageW.Call(control, wmSetFont, font, 1)
			if id != 0 {
				ui.controls[id] = control
			}
		}
		return control
	}

	create(0, "STATIC", "规则列表", 0, 20, 16, 300, 22)
	create(idRuleList, "LISTBOX", "", wsTabStop|wsBorder|wsVScroll|lbsNotify|lbsNoIntegralHeight, 20, 42, 300, 500)
	create(idAddRule, "BUTTON", "新增", wsTabStop|bsPushButton, 20, 552, 55, 30)
	create(idDeleteRule, "BUTTON", "删除", wsTabStop|bsPushButton, 82, 552, 55, 30)
	create(idMoveUp, "BUTTON", "上移", wsTabStop|bsPushButton, 144, 552, 55, 30)
	create(idMoveDown, "BUTTON", "下移", wsTabStop|bsPushButton, 206, 552, 55, 30)

	create(0, "STATIC", "关键词", 0, 350, 20, 90, 22)
	create(idKeyword, "EDIT", "", wsTabStop|wsBorder|esAutoHScroll, 450, 16, 490, 28)
	create(0, "STATIC", "匹配模式", 0, 350, 62, 90, 22)
	match := create(idMatchMode, "COMBOBOX", "", wsTabStop|wsVScroll|cbsDropdownList, 450, 58, 180, 200)
	addComboItems(match, []string{"精准", "模糊"})
	create(idCaseSensitive, "BUTTON", "大小写敏感", wsTabStop|bsAutoCheckbox, 660, 60, 150, 24)

	create(0, "STATIC", "触发区域", 0, 350, 104, 90, 22)
	create(idAreaFriend, "BUTTON", "QQ 好友", wsTabStop|bsAutoCheckbox, 450, 100, 110, 24)
	create(idAreaGroup, "BUTTON", "群聊", wsTabStop|bsAutoCheckbox, 570, 100, 90, 24)
	create(idAreaChannel, "BUTTON", "频道消息", wsTabStop|bsAutoCheckbox, 670, 100, 110, 24)
	create(idAreaChannelPrivate, "BUTTON", "频道私信", wsTabStop|bsAutoCheckbox, 790, 100, 110, 24)

	create(0, "STATIC", "回复类型", 0, 350, 146, 90, 22)
	reply := create(idReplyType, "COMBOBOX", "", wsTabStop|wsVScroll|cbsDropdownList, 450, 142, 180, 250)
	addComboItems(reply, []string{"普通消息", "Markdown", "语音", "视频", "文件"})
	create(0, "STATIC", "回复内容", 0, 350, 188, 90, 22)
	create(idReplyContent, "EDIT", "", wsTabStop|wsBorder|wsVScroll|esMultiline|esAutoVScroll|esWantReturn, 450, 184, 490, 320)
	create(idSaveRule, "BUTTON", "保存规则", wsTabStop|bsPushButton, 450, 522, 100, 32)
	create(idStatus, "STATIC", "", 0, 350, 580, 590, 36)

	ui.setDraft(NewRuleDraft())
	ui.refreshList(-1)
	return ui
}

func (ui *settingsUI) handleCommand(id, notification int) {
	switch {
	case id == idRuleList && notification == lbnSelChange:
		index := ui.currentListSelection()
		rules := ui.controller.Rules()
		if index >= 0 && index < len(rules) {
			ui.selected = index
			ui.setDraft(DraftFromRule(rules[index]))
			ui.setStatus("")
		}
	case id == idReplyType && notification == cbnSelChange:
		ui.updateMediaAreaState()
	case id == idAddRule && notification == bnClicked:
		if err := ui.controller.Add(ui.draft()); err != nil {
			ui.setStatus(err.Error())
			return
		}
		ui.refreshList(len(ui.controller.Rules()) - 1)
		ui.setStatus("规则已新增并保存")
	case id == idSaveRule && notification == bnClicked:
		if ui.selected < 0 {
			ui.setStatus("请先选择规则，或使用新增按钮")
			return
		}
		if err := ui.controller.Update(ui.selected, ui.draft()); err != nil {
			ui.setStatus(err.Error())
			return
		}
		ui.refreshList(ui.selected)
		ui.setStatus("规则已保存")
	case id == idDeleteRule && notification == bnClicked:
		if ui.selected < 0 {
			ui.setStatus("请先选择要删除的规则")
			return
		}
		index := ui.selected
		if err := ui.controller.Delete(index); err != nil {
			ui.setStatus(err.Error())
			return
		}
		if index >= len(ui.controller.Rules()) {
			index--
		}
		ui.refreshList(index)
		if index < 0 {
			ui.setDraft(NewRuleDraft())
		}
		ui.setStatus("规则已删除")
	case (id == idMoveUp || id == idMoveDown) && notification == bnClicked:
		if ui.selected < 0 {
			ui.setStatus("请先选择要移动的规则")
			return
		}
		delta := 1
		if id == idMoveUp {
			delta = -1
		}
		index, err := ui.controller.Move(ui.selected, delta)
		if err != nil {
			ui.setStatus(err.Error())
			return
		}
		ui.refreshList(index)
		ui.setStatus("规则顺序已保存")
	}
}

func (ui *settingsUI) refreshList(selected int) {
	list := ui.controls[idRuleList]
	procSendMessageW.Call(list, lbResetContent, 0, 0)
	rules := ui.controller.Rules()
	for _, rule := range rules {
		value := utf16Ptr(RuleSummary(rule))
		procSendMessageW.Call(list, lbAddString, 0, uintptr(unsafe.Pointer(value)))
	}
	if selected >= 0 && selected < len(rules) {
		procSendMessageW.Call(list, lbSetCurSel, uintptr(selected), 0)
		ui.selected = selected
		ui.setDraft(DraftFromRule(rules[selected]))
	} else {
		ui.selected = -1
	}
}

func (ui *settingsUI) currentListSelection() int {
	result, _, _ := procSendMessageW.Call(ui.controls[idRuleList], lbGetCurSel, 0, 0)
	return int(int32(result))
}

func (ui *settingsUI) draft() RuleDraft {
	return RuleDraft{
		Keyword:            controlText(ui.controls[idKeyword]),
		MatchMode:          []MatchMode{MatchExact, MatchFuzzy}[safeComboIndex(ui.controls[idMatchMode], 2)],
		CaseSensitive:      controlChecked(ui.controls[idCaseSensitive]),
		AreaFriend:         controlChecked(ui.controls[idAreaFriend]),
		AreaGroup:          controlChecked(ui.controls[idAreaGroup]),
		AreaChannel:        controlChecked(ui.controls[idAreaChannel]),
		AreaChannelPrivate: controlChecked(ui.controls[idAreaChannelPrivate]),
		ReplyType:          []ReplyType{ReplyText, ReplyMarkdown, ReplyAudio, ReplyVideo, ReplyFile}[safeComboIndex(ui.controls[idReplyType], 5)],
		Content:            controlText(ui.controls[idReplyContent]),
	}
}

func (ui *settingsUI) setDraft(draft RuleDraft) {
	setControlText(ui.controls[idKeyword], draft.Keyword)
	matchIndex := 0
	if draft.MatchMode == MatchFuzzy {
		matchIndex = 1
	}
	procSendMessageW.Call(ui.controls[idMatchMode], cbSetCurSel, uintptr(matchIndex), 0)
	setControlChecked(ui.controls[idCaseSensitive], draft.CaseSensitive)
	setControlChecked(ui.controls[idAreaFriend], draft.AreaFriend)
	setControlChecked(ui.controls[idAreaGroup], draft.AreaGroup)
	setControlChecked(ui.controls[idAreaChannel], draft.AreaChannel)
	setControlChecked(ui.controls[idAreaChannelPrivate], draft.AreaChannelPrivate)
	replyIndex := map[ReplyType]int{ReplyText: 0, ReplyMarkdown: 1, ReplyAudio: 2, ReplyVideo: 3, ReplyFile: 4}[draft.ReplyType]
	procSendMessageW.Call(ui.controls[idReplyType], cbSetCurSel, uintptr(replyIndex), 0)
	setControlText(ui.controls[idReplyContent], draft.Content)
	ui.updateMediaAreaState()
}

func (ui *settingsUI) updateMediaAreaState() {
	media := safeComboIndex(ui.controls[idReplyType], 5) >= 2
	for _, id := range []int{idAreaChannel, idAreaChannelPrivate} {
		if media {
			setControlChecked(ui.controls[id], false)
		}
		enabled := uintptr(1)
		if media {
			enabled = 0
		}
		procEnableWindow.Call(ui.controls[id], enabled)
	}
}

func (ui *settingsUI) setStatus(text string) {
	setControlText(ui.controls[idStatus], text)
}

func addComboItems(control uintptr, items []string) {
	for _, item := range items {
		value := utf16Ptr(item)
		procSendMessageW.Call(control, cbAddString, 0, uintptr(unsafe.Pointer(value)))
	}
}

func safeComboIndex(control uintptr, count int) int {
	result, _, _ := procSendMessageW.Call(control, cbGetCurSel, 0, 0)
	index := int(int32(result))
	if index < 0 || index >= count {
		return 0
	}
	return index
}

func controlChecked(control uintptr) bool {
	result, _, _ := procSendMessageW.Call(control, bmGetCheck, 0, 0)
	return result == bstChecked
}

func setControlChecked(control uintptr, checked bool) {
	value := uintptr(bstUnchecked)
	if checked {
		value = bstChecked
	}
	procSendMessageW.Call(control, bmSetCheck, value, 0)
}

func controlText(control uintptr) string {
	length, _, _ := procGetWindowTextLength.Call(control)
	buffer := make([]uint16, int(length)+1)
	procGetWindowTextW.Call(control, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
	return syscall.UTF16ToString(buffer)
}

func setControlText(control uintptr, text string) {
	procSetWindowTextW.Call(control, uintptr(unsafe.Pointer(utf16Ptr(text))))
}

func utf16Ptr(value string) *uint16 {
	pointer, _ := syscall.UTF16PtrFromString(value)
	return pointer
}

func showSettingsError(hwnd uintptr, text string) {
	procMessageBoxW.Call(hwnd, uintptr(unsafe.Pointer(utf16Ptr(text))), uintptr(unsafe.Pointer(utf16Ptr(PluginName))), mbOK|mbIconError)
}

func startHostWatcher(hostPID uint32) {
	handle, _, _ := openProcess.Call(synchronize, 0, uintptr(hostPID))
	if handle == 0 {
		return
	}
	go func() {
		defer closeHandle.Call(handle)
		waitSingleObject.Call(handle, 0xffffffff)
		os.Exit(0)
	}()
}

//go:build windows

package main

import (
	"errors"
	"os"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"unsafe"
)

const (
	settingsBaseWidth   = 520
	settingsBaseHeight  = 260
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
	idSettingsPort = 1001 + iota
	idCheckPort
	idEnableService
	idDisableService
	idOpenSettingsPage
	idServiceStatus
)

const (
	synchronize         = 0x00100000
	wmCreate            = 0x0001
	wmDestroy           = 0x0002
	wmClose             = 0x0010
	wmSetFont           = 0x0030
	wmCommand           = 0x0111
	wmCtlColorEdit      = 0x0133
	wmCtlColorBtn       = 0x0135
	wmCtlColorStatic    = 0x0138
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
	whiteBrush          = 0
	settingsWhiteColor  = 0x00ffffff
	settingsBlackColor  = 0x00000000
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
	procSetBkColor          = gdi32.NewProc("SetBkColor")
	procSetTextColor        = gdi32.NewProc("SetTextColor")
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
	controls map[int]uintptr
}

var settingsNative = struct {
	sync.Mutex
	hwnd    uintptr
	done    chan struct{}
	closing bool
	ui      *settingsUI
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
		settingsNative.ui = nil
		close(done)
		settingsNative.Unlock()
	}()

	instance, _, _ := procGetModuleHandleW.Call(0)
	className := utf16Ptr("BeeKeywordReplySettingsWindow")
	title := utf16Ptr(PluginName + " 设置服务")
	cursor, _, _ := procLoadCursorW.Call(0, idcArrow)
	icon, _, _ := procLoadIconW.Call(0, idiApplication)
	class := wndClassEx{Size: uint32(unsafe.Sizeof(wndClassEx{})), WndProc: settingsWndProc, Instance: instance, Icon: icon, Cursor: cursor, Background: settingsWhiteBrush(), ClassName: uintptr(unsafe.Pointer(className)), IconSmall: icon}
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
		ui := createSettingsUI(hwnd)
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
	case wmCtlColorEdit, wmCtlColorBtn, wmCtlColorStatic:
		return paintSettingsControlWhite(wparam)
	}
	result, _, _ := procDefWindowProcW.Call(hwnd, uintptr(message), wparam, lparam)
	return result
}

func createSettingsUI(hwnd uintptr) *settingsUI {
	dpi := settingsDPI()
	s := func(value int) int { return scaleDPI(value, dpi) }
	ui := &settingsUI{controls: make(map[int]uintptr)}
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

	create(0, "STATIC", "本地 HTTP 设置服务", 0, 20, 18, 460, 24)
	create(0, "STATIC", "服务不会自动启动。只有点击“启用服务”后才会监听 127.0.0.1 并打开浏览器。", 0, 20, 48, 470, 36)
	create(0, "STATIC", "端口", 0, 20, 98, 60, 24)
	create(idSettingsPort, "EDIT", "6655", wsTabStop|wsBorder|esAutoHScroll, 82, 94, 120, 28)
	create(idCheckPort, "BUTTON", "检测端口", wsTabStop|bsPushButton, 216, 92, 92, 32)
	create(idEnableService, "BUTTON", "启用服务", wsTabStop|bsPushButton, 20, 142, 92, 34)
	create(idDisableService, "BUTTON", "停用服务", wsTabStop|bsPushButton, 124, 142, 92, 34)
	create(idOpenSettingsPage, "BUTTON", "打开网页", wsTabStop|bsPushButton, 228, 142, 92, 34)
	create(idServiceStatus, "STATIC", "", 0, 20, 194, 470, 42)

	ui.refreshStatus()
	return ui
}

func (ui *settingsUI) handleCommand(id, notification int) {
	if notification != bnClicked {
		return
	}
	switch id {
	case idCheckPort:
		ui.checkPort()
	case idEnableService:
		ui.enableService()
	case idDisableService:
		ui.disableService()
	case idOpenSettingsPage:
		ui.openSettingsPage()
	}
}

func (ui *settingsUI) selectedPort() (int, error) {
	return parseSettingsPort(controlText(ui.controls[idSettingsPort]))
}

func (ui *settingsUI) checkPort() {
	port, err := ui.selectedPort()
	if err != nil {
		ui.setStatus(err.Error())
		return
	}
	status := settingsWebServiceStatus()
	if status.Running && status.Port == port {
		ui.setStatus("HTTP 服务正在使用该端口")
		return
	}
	if err := checkSettingsPortAvailable(port); err != nil {
		ui.setStatus(err.Error())
		return
	}
	ui.setStatus("端口可用")
	ui.refreshStatus()
}

func (ui *settingsUI) enableService() {
	port, err := ui.selectedPort()
	if err != nil {
		ui.setStatus(err.Error())
		return
	}
	rawURL, err := startSettingsWebService(port)
	if err != nil {
		ui.refreshStatus()
		ui.setStatus(err.Error())
		return
	}
	ui.refreshStatus()
	ui.setStatus("HTTP 服务已启动：" + rawURL)
}

func (ui *settingsUI) disableService() {
	if err := stopSettingsWebService(); err != nil {
		ui.setStatus("停用服务失败：" + err.Error())
		return
	}
	ui.refreshStatus()
	ui.setStatus("HTTP 服务已停用")
}

func (ui *settingsUI) openSettingsPage() {
	status := settingsWebServiceStatus()
	if !status.Running || status.URL == "" {
		ui.setStatus("HTTP 服务未运行")
		return
	}
	if err := openDefaultBrowser(status.URL); err != nil {
		ui.setStatus("打开浏览器失败：" + err.Error())
		return
	}
	ui.setStatus("已打开设置网页")
}

func (ui *settingsUI) refreshStatus() {
	status := settingsWebServiceStatus()
	if status.Running {
		setControlText(ui.controls[idSettingsPort], strconv.Itoa(status.Port))
		ui.setStatus("HTTP 服务运行中：" + status.URL)
		enableControl(ui.controls[idSettingsPort], false)
		enableControl(ui.controls[idCheckPort], false)
		enableControl(ui.controls[idEnableService], false)
		enableControl(ui.controls[idDisableService], true)
		enableControl(ui.controls[idOpenSettingsPage], true)
		if controlText(ui.controls[idServiceStatus]) == "" {
			ui.setStatus("HTTP 服务运行中：" + status.URL)
		}
		return
	}
	enableControl(ui.controls[idSettingsPort], true)
	enableControl(ui.controls[idCheckPort], true)
	enableControl(ui.controls[idEnableService], true)
	enableControl(ui.controls[idDisableService], false)
	enableControl(ui.controls[idOpenSettingsPage], false)
	if controlText(ui.controls[idServiceStatus]) == "" {
		ui.setStatus("HTTP 服务未运行。默认端口为 6655。")
	}
}

func (ui *settingsUI) setStatus(text string) {
	setControlText(ui.controls[idServiceStatus], text)
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

func paintSettingsControlWhite(dc uintptr) uintptr {
	procSetTextColor.Call(dc, settingsBlackColor)
	procSetBkColor.Call(dc, settingsWhiteColor)
	return settingsWhiteBrush()
}

func settingsWhiteBrush() uintptr {
	brush, _, _ := procGetStockObject.Call(whiteBrush)
	return brush
}

func enableControl(control uintptr, enabled bool) {
	value := uintptr(0)
	if enabled {
		value = 1
	}
	procEnableWindow.Call(control, value)
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

package main

import (
	_ "embed"
	"flag"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/getlantern/systray"
	"github.com/lxn/win"
)

var (
	user32           = syscall.NewLazyDLL("user32.dll")
	gdi32            = syscall.NewLazyDLL("gdi32.dll")
	setLayeredWindow = user32.NewProc("SetLayeredWindowAttributes")
	createFont       = gdi32.NewProc("CreateFontW")
	drawText         = user32.NewProc("DrawTextW")
	fillRect         = user32.NewProc("FillRect")
	createSolidBrush = gdi32.NewProc("CreateSolidBrush")

	// Настройки
	fontSize   int
	fontName   string
	fontWeight int
	textColor  win.COLORREF
	posX, posY int

	// Глобальный HWND для закрытия окна при выходе из трея
	clockHwnd win.HWND
)

//go:embed icon.ico
var iconBytes []byte

const (
	LWA_COLORKEY = 0x00000001
	DT_LEFT      = 0x00000000
	DT_NOCLIP    = 0x00000100
	clockWidth   = 400
	clockHeight  = 120
)

func getColor(name string) win.COLORREF {
	switch strings.ToLower(name) {
	case "red":
		return win.COLORREF(win.RGB(255, 0, 0))
	case "blue":
		return win.COLORREF(win.RGB(0, 0, 255))
	case "white":
		return win.COLORREF(win.RGB(255, 255, 255))
	case "yellow":
		return win.COLORREF(win.RGB(255, 255, 0))
	case "cyan":
		return win.COLORREF(win.RGB(0, 255, 255))
	case "magenta":
		return win.COLORREF(win.RGB(255, 0, 255))
	case "gray":
		return win.COLORREF(win.RGB(128, 128, 128))
	default:
		return win.COLORREF(win.RGB(0, 255, 0)) // green default
	}
}

func updatePosition(hwnd win.HWND) {
	finalX, finalY := posX, posY

	// Если координаты не заданы (равны -1), считаем автоматически для угла
	if posX == -1 || posY == -1 {
		sw := win.GetSystemMetrics(win.SM_CXSCREEN)
		sh := win.GetSystemMetrics(win.SM_CYSCREEN)
		finalX = int(sw) - clockWidth - 30
		finalY = int(sh) - clockHeight - 40
	}

	win.SetWindowPos(hwnd, win.HWND_TOPMOST, int32(finalX), int32(finalY), clockWidth, clockHeight, win.SWP_SHOWWINDOW)
}

func main() {
	colorFlag := flag.String("color", "green", "Цвет")
	flag.IntVar(&fontSize, "size", 48, "Размер шрифта")
	flag.StringVar(&fontName, "font", "Consolas", "Шрифт")
	flag.IntVar(&fontWeight, "weight", 400, "Жирность (100-900)")
	flag.IntVar(&posX, "x", -1, "Позиция X (-1 для авто)")
	flag.IntVar(&posY, "y", -1, "Позиция Y (-1 для авто)")
	flag.Parse()

	textColor = getColor(*colorFlag)

	// Запускаем systray. Он блокирует текущий (main) поток.
	systray.Run(onReady, onExit)
}

func onReady() {
	systray.SetTitle("OSD Clock")
	systray.SetTooltip("Прозрачные Часы")

	systray.SetIcon(iconBytes)

	mQuit := systray.AddMenuItem("Выход", "Закрыть часы")

	// Обработчик клика по кнопке Выход
	go func() {
		<-mQuit.ClickedCh
		systray.Quit()
	}()

	// Само Win32-окно часов запускаем в отдельном потоке
	go startClockWindow()
}

func onExit() {
	// Посылаем сигнал закрытия окну, если оно было создано
	if clockHwnd != 0 {
		win.PostMessage(clockHwnd, win.WM_CLOSE, 0, 0)
	}
}

func startClockWindow() {
	// Привязываем горутину к потоку ОС, так как цикл обработки сообщений Windows
	// должен работать в том же потоке, где было создано окно.
	runtime.LockOSThread()

	className := syscall.StringToUTF16Ptr("ClockWindowClass")
	hInstance := win.GetModuleHandle(nil)

	wndClass := win.WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(win.WNDCLASSEX{})),
		LpfnWndProc:   syscall.NewCallback(wndProc),
		HInstance:     hInstance,
		LpszClassName: className,
		HCursor:       win.LoadCursor(0, win.MAKEINTRESOURCE(win.IDC_ARROW)),
	}
	win.RegisterClassEx(&wndClass)

	clockHwnd = win.CreateWindowEx(
		win.WS_EX_LAYERED|win.WS_EX_TRANSPARENT|win.WS_EX_TOPMOST|win.WS_EX_TOOLWINDOW,
		className,
		syscall.StringToUTF16Ptr("OSD Clock"),
		win.WS_POPUP,
		0, 0, clockWidth, clockHeight,
		0, 0, hInstance, nil,
	)

	updatePosition(clockHwnd)
	setLayeredWindow.Call(uintptr(clockHwnd), 0, 255, LWA_COLORKEY)

	// Таймер на перерисовку (1000 мс)
	win.SetTimer(clockHwnd, 1, 1000, 0)

	// Главный цикл сообщений для окна часов
	var msg win.MSG
	for win.GetMessage(&msg, 0, 0, 0) > 0 {
		win.TranslateMessage(&msg)
		win.DispatchMessage(&msg)
	}
}

func wndProc(hwnd win.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case win.WM_DISPLAYCHANGE:
		updatePosition(hwnd)
		return 0
	case win.WM_TIMER:
		// Вызываем перерисовку каждую секунду
		win.InvalidateRect(hwnd, nil, true)
		return 0
	case win.WM_PAINT:
		var ps win.PAINTSTRUCT
		hdc := win.BeginPaint(hwnd, &ps)

		rect := win.RECT{Left: 0, Top: 0, Right: clockWidth, Bottom: clockHeight}

		// Заливка черным фоном (который потом становится прозрачным благодаря LWA_COLORKEY)
		hBrush, _, _ := createSolidBrush.Call(uintptr(0))
		fillRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(&rect)), hBrush)
		win.DeleteObject(win.HGDIOBJ(hBrush))

		// Создаем шрифт и рисуем текст
		hFont, _, _ := createFont.Call(
			uintptr(fontSize), 0, 0, 0, uintptr(fontWeight), 0, 0, 0,
			win.DEFAULT_CHARSET, 0, 0, 4, 0,
			uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(fontName))),
		)
		oldFont := win.SelectObject(hdc, win.HGDIOBJ(hFont))
		win.SetTextColor(hdc, textColor)
		win.SetBkMode(hdc, win.TRANSPARENT)

		t := time.Now().Format("15:04")
		drawText.Call(uintptr(hdc), uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(t))),
			uintptr(len(t)), uintptr(unsafe.Pointer(&rect)), DT_LEFT|DT_NOCLIP)

		// Очистка
		win.SelectObject(hdc, oldFont)
		win.DeleteObject(win.HGDIOBJ(hFont))
		win.EndPaint(hwnd, &ps)
		return 0
	case win.WM_DESTROY:
		win.PostQuitMessage(0)
		return 0
	}
	return win.DefWindowProc(hwnd, msg, wParam, lParam)
}

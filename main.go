package main

import (
	_ "embed"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"github.com/pelletier/go-toml/v2"
	"github.com/getlantern/systray"
	"github.com/lxn/win"
)

//go:embed icon.ico
var iconBytes []byte

// ============================================================================
// 1. САМОДОКУМЕНТИРУЕМЫЕ КОНСТАНТЫ И ДОМЕННЫЕ ТИПЫ
// ============================================================================

const (
	autoPosition = -1

	// Геометрия OSD-часов
	defaultClockWidth  = 400
	defaultClockHeight = 120
	screenMarginRight  = 30
	screenMarginBottom = 40

	// Win32 API константы
	lwaColorKey = 0x00000001
	dtLeft      = 0x00000000
	dtNoClip    = 0x00000100
	srccopy     = 0x00CC0020 // Raster operation code для BitBlt

	// Настройки отрисовки
	redrawIntervalMs  = 1000 // Интервал обновления (1 секунда)
	timerIDRender     = 1
	defaultColorName  = "green"
	defaultFontName   = "Consolas"
	defaultFontSize   = 48
	defaultFontWeight = 400
	defaultConfigFile = "config.toml"
)

// TimeFormat определяет формат отображения времени (12-часовой или 24-часовой).
type TimeFormat string

const (
	Format24H TimeFormat = "24h"
	Format12H TimeFormat = "12h"
)

// Config инкапсулирует пользовательские параметры запуска.
type Config struct {
	FontName   string       `toml:"font_name"`
	FontSize   int          `toml:"font_size"`
	FontWeight int          `toml:"font_weight"`
	Color      string       `toml:"color"`
	TextColor  win.COLORREF `toml:"-"`
	PositionX  int          `toml:"pos_x"`
	PositionY  int          `toml:"pos_y"`
	TimeFormat TimeFormat   `toml:"time_format"` // "24h" или "12h"
}

// WindowBounds — доменное представление позиции и размеров окна.
type WindowBounds struct {
	X, Y          int32
	Width, Height int32
}

// TimeBuffer обеспечивает нулевые аллокации (Zero-Allocation) при формировании
// строки времени для Win32 API (UTF-16) в hot-path отрисовки.
type TimeBuffer struct {
	// Формат "HH:MM\0" состоит из 5 символов + null-терминатор
	utf16Buf [6]uint16
}

// FormatNow24h записывает текущие часы и минуты (24h) напрямую в буфер UTF-16.
func (tb *TimeBuffer) FormatNow24h() *uint16 {
	now := time.Now()
	hour, min, _ := now.Clock()

	tb.utf16Buf[0] = uint16('0' + hour/10)
	tb.utf16Buf[1] = uint16('0' + hour%10)
	tb.utf16Buf[2] = uint16(':')
	tb.utf16Buf[3] = uint16('0' + min/10)
	tb.utf16Buf[4] = uint16('0' + min%10)
	tb.utf16Buf[5] = 0 // Null-terminator для WinAPI C-strings

	return &tb.utf16Buf[0]
}

// FormatNow12h записывает текущие часы и минуты в 12-часовом формате (без AM/PM) в буфер UTF-16.
func (tb *TimeBuffer) FormatNow12h() *uint16 {
	now := time.Now()
	hour, min, _ := now.Clock()

	hour12 := hour % 12
	if hour12 == 0 {
		hour12 = 12
	}

	tb.utf16Buf[0] = uint16('0' + hour12/10)
	tb.utf16Buf[1] = uint16('0' + hour12%10)
	tb.utf16Buf[2] = uint16(':')
	tb.utf16Buf[3] = uint16('0' + min/10)
	tb.utf16Buf[4] = uint16('0' + min%10)
	tb.utf16Buf[5] = 0 // Null-terminator

	return &tb.utf16Buf[0]
}

// FormatNow формирует время в буфере в соответствии с выбранным форматом.
func (tb *TimeBuffer) FormatNow(fmt TimeFormat) *uint16 {
	if fmt == Format12H {
		return tb.FormatNow12h()
	}
	return tb.FormatNow24h()
}

// ============================================================================
// 2. ИНКАПСУЛЯЦИЯ WIN32 DLL
// ============================================================================

type win32API struct {
	setLayeredWindow *syscall.LazyProc
	createFont       *syscall.LazyProc
	drawText         *syscall.LazyProc
	fillRect         *syscall.LazyProc
	createSolidBrush *syscall.LazyProc
	bitBlt           *syscall.LazyProc
}

func newWin32API() *win32API {
	user32 := syscall.NewLazyDLL("user32.dll")
	gdi32 := syscall.NewLazyDLL("gdi32.dll")

	return &win32API{
		setLayeredWindow: user32.NewProc("SetLayeredWindowAttributes"),
		createFont:       gdi32.NewProc("CreateFontW"),
		drawText:         user32.NewProc("DrawTextW"),
		fillRect:         user32.NewProc("FillRect"),
		createSolidBrush: gdi32.NewProc("CreateSolidBrush"),
		bitBlt:           gdi32.NewProc("BitBlt"),
	}
}

var winAPI = newWin32API()

// ============================================================================
// 3. ДОМЕННЫЙ ОБЪЕКТ ЧАСОВ
// ============================================================================

type ClockWindow struct {
	config Config
	hwnd   win.HWND

	// Кэшированные Win32 GDI-ресурсы
	fontHandle  win.HFONT
	brushHandle win.HBRUSH

	// Предварительно выделенный буфер времени
	timeBuffer TimeBuffer

	isVisible bool
}

func NewClockWindow(cfg Config) *ClockWindow {
	return &ClockWindow{
		config:    cfg,
		isVisible: true,
	}
}

func (cw *ClockWindow) CalculateBounds() WindowBounds {
	x := cw.config.PositionX
	y := cw.config.PositionY

	if x == autoPosition || y == autoPosition {
		screenWidth := int(win.GetSystemMetrics(win.SM_CXSCREEN))
		screenHeight := int(win.GetSystemMetrics(win.SM_CYSCREEN))

		if x == autoPosition {
			x = screenWidth - defaultClockWidth - screenMarginRight
		}
		if y == autoPosition {
			y = screenHeight - defaultClockHeight - screenMarginBottom
		}
	}

	return WindowBounds{
		X:      int32(x),
		Y:      int32(y),
		Width:  defaultClockWidth,
		Height: defaultClockHeight,
	}
}

func (cw *ClockWindow) UpdatePosition() {
	if cw.hwnd == 0 {
		return
	}
	bounds := cw.CalculateBounds()
	ok := win.SetWindowPos(
		cw.hwnd,
		win.HWND_TOPMOST,
		bounds.X, bounds.Y,
		bounds.Width, bounds.Height,
		win.SWP_SHOWWINDOW,
	)
	if !ok {
		log.Printf("[WARN] Не удалось обновить позицию окна: err=%d", win.GetLastError())
	}
}

func (cw *ClockWindow) ToggleVisibility(menuItem *systray.MenuItem) {
	cw.isVisible = !cw.isVisible

	if cw.isVisible {
		win.ShowWindow(cw.hwnd, win.SW_SHOW)
		menuItem.Check()
	} else {
		win.ShowWindow(cw.hwnd, win.SW_HIDE)
		menuItem.Uncheck()
	}
}

func (cw *ClockWindow) InitGDIResources() error {
	fontNamePtr, err := syscall.UTF16PtrFromString(cw.config.FontName)
	if err != nil {
		log.Printf("[WARN] Некорректное имя шрифта '%s', сброс на дефолтный: %v", cw.config.FontName, err)
		fontNamePtr, _ = syscall.UTF16PtrFromString(defaultFontName)
	}

	hFont, _, _ := winAPI.createFont.Call(
		uintptr(cw.config.FontSize), 0, 0, 0,
		uintptr(cw.config.FontWeight), 0, 0, 0,
		win.DEFAULT_CHARSET, 0, 0, 4, 0,
		uintptr(unsafe.Pointer(fontNamePtr)),
	)
	if hFont == 0 {
		return fmt.Errorf("ошибка вызова CreateFontW (code: %d)", win.GetLastError())
	}
	cw.fontHandle = win.HFONT(hFont)

	hBrush, _, _ := winAPI.createSolidBrush.Call(uintptr(0))
	if hBrush == 0 {
		return fmt.Errorf("ошибка вызова CreateSolidBrush (code: %d)", win.GetLastError())
	}
	cw.brushHandle = win.HBRUSH(hBrush)

	return nil
}

func (cw *ClockWindow) FreeGDIResources() {
	if cw.fontHandle != 0 {
		win.DeleteObject(win.HGDIOBJ(cw.fontHandle))
		cw.fontHandle = 0
	}
	if cw.brushHandle != 0 {
		win.DeleteObject(win.HGDIOBJ(cw.brushHandle))
		cw.brushHandle = 0
	}
}

// ============================================================================
// 4. КОНФИГУРАЦИЯ И ВСПОМОГАТЕЛЬНЫЙ ФУНКЦИОНАЛ
// ============================================================================

var colorPalette = map[string]win.COLORREF{
	"red":     win.RGB(255, 0, 0),
	"blue":    win.RGB(0, 0, 255),
	"white":   win.RGB(255, 255, 255),
	"yellow":  win.RGB(255, 255, 0),
	"cyan":    win.RGB(0, 255, 255),
	"magenta": win.RGB(255, 0, 255),
	"gray":    win.RGB(128, 128, 128),
	"green":   win.RGB(0, 255, 0),
}

func parseColor(colorName string) win.COLORREF {
	if color, exists := colorPalette[colorName]; exists {
		return color
	}
	return colorPalette[defaultColorName]
}

func loadConfig() Config {
	cfg := Config{
		FontName:   defaultFontName,
		FontSize:   defaultFontSize,
		FontWeight: defaultFontWeight,
		Color:      defaultColorName,
		PositionX:  autoPosition,
		PositionY:  autoPosition,
		TimeFormat: Format24H,
	}

	if fileData, err := os.ReadFile(defaultConfigFile); err == nil {
		if err := toml.Unmarshal(fileData, &cfg); err != nil {
			log.Printf("[WARN] Ошибка парсинга %s, применяются дефолтные параметры: %v", defaultConfigFile, err)
		}
	}

	flagColor := flag.String("color", "", "Цвет текста")
	flagFontSize := flag.Int("size", 0, "Размер шрифта")
	flagFontName := flag.String("font", "", "Название шрифта")
	flagFontWeight := flag.Int("weight", 0, "Жирность шрифта (100-900)")
	flagPosX := flag.Int("x", 0, "Позиция X (-1 для авто)")
	flagPosY := flag.Int("y", 0, "Позиция Y (-1 для авто)")
	flagFormat := flag.String("format", "", "Формат времени: 24h или 12h")
	flag.Parse()

	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "color":
			cfg.Color = *flagColor
		case "size":
			cfg.FontSize = *flagFontSize
		case "font":
			cfg.FontName = *flagFontName
		case "weight":
			cfg.FontWeight = *flagFontWeight
		case "x":
			cfg.PositionX = *flagPosX
		case "y":
			cfg.PositionY = *flagPosY
		case "format":
			if *flagFormat == string(Format12H) {
				cfg.TimeFormat = Format12H
			} else {
				cfg.TimeFormat = Format24H
			}
		}
	})

	cfg.TextColor = parseColor(cfg.Color)
	return cfg
}

var clockApp *ClockWindow

// ============================================================================
// 5. ВХОДНАЯ ТОЧКА И СИСТЕМНЫЙ ТРЕЙ
// ============================================================================

func main() {
	config := loadConfig()
	clockApp = NewClockWindow(config)

	systray.Run(onReady, onExit)
}

func onReady() {
	systray.SetIcon(iconBytes)
	systray.SetTitle("Desktop Clock")
	systray.SetTooltip("Прозрачные Часы")

	mToggleVisible := systray.AddMenuItem("Показать часы", "")
	mToggleVisible.Check()

	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Выход", "")

	go func() {
		for {
			select {
			case <-mToggleVisible.ClickedCh:
				clockApp.ToggleVisibility(mToggleVisible)
			case <-mQuit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()

	go startClockWindow()
}

func onExit() {
	if clockApp != nil && clockApp.hwnd != 0 {
		win.PostMessage(clockApp.hwnd, win.WM_CLOSE, 0, 0)
	}
}

// ============================================================================
// 6. WIN32 EVENT LOOP И ДВОЙНАЯ БУФЕРИЗАЦИЯ
// ============================================================================

func startClockWindow() {
	runtime.LockOSThread()

	className, err := syscall.UTF16PtrFromString("ClockWindowClass")
	if err != nil {
		log.Fatalf("[FATAL] Ошибка преобразования имени класса: %v", err)
	}
	windowTitle, err := syscall.UTF16PtrFromString("OSD Clock")
	if err != nil {
		log.Fatalf("[FATAL] Ошибка преобразования заголовка окна: %v", err)
	}

	hInstance := win.GetModuleHandle(nil)

	wndClass := win.WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(win.WNDCLASSEX{})),
		LpfnWndProc:   syscall.NewCallback(wndProc),
		HInstance:     hInstance,
		LpszClassName: className,
		HCursor:       win.LoadCursor(0, win.MAKEINTRESOURCE(win.IDC_ARROW)),
	}

	if atom := win.RegisterClassEx(&wndClass); atom == 0 {
		log.Fatalf("[FATAL] Не удалось зарегистрировать класс окна. Код ошибки: %d", win.GetLastError())
	}

	bounds := clockApp.CalculateBounds()

	hwnd := win.CreateWindowEx(
		win.WS_EX_LAYERED|win.WS_EX_TRANSPARENT|win.WS_EX_TOPMOST|win.WS_EX_TOOLWINDOW,
		className,
		windowTitle,
		win.WS_POPUP,
		bounds.X, bounds.Y, bounds.Width, bounds.Height,
		0, 0, hInstance, nil,
	)

	if hwnd == 0 {
		log.Fatalf("[FATAL] Ошибка создания Win32 окна. Код ошибки: %d", win.GetLastError())
	}

	clockApp.hwnd = hwnd

	if err := clockApp.InitGDIResources(); err != nil {
		log.Fatalf("[FATAL] Ошибка инициализации GDI ресурсов: %v", err)
	}

	clockApp.UpdatePosition()

	winAPI.setLayeredWindow.Call(uintptr(hwnd), 0, 255, lwaColorKey)

	if timerID := win.SetTimer(hwnd, timerIDRender, redrawIntervalMs, 0); timerID == 0 {
		log.Printf("[WARN] Сбой инициализации Win32 таймера. Код ошибки: %d", win.GetLastError())
	}

	var msg win.MSG
	for win.GetMessage(&msg, 0, 0, 0) > 0 {
		win.TranslateMessage(&msg)
		win.DispatchMessage(&msg)
	}
}

func wndProc(hwnd win.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case win.WM_DISPLAYCHANGE:
		clockApp.UpdatePosition()
		return 0

	case win.WM_TIMER:
		win.InvalidateRect(hwnd, nil, true)
		return 0

	case win.WM_PAINT:
		renderClockFrameBuffered(hwnd)
		return 0

	case win.WM_DESTROY:
		clockApp.FreeGDIResources()
		win.PostQuitMessage(0)
		return 0
	}

	return win.DefWindowProc(hwnd, msg, wParam, lParam)
}

func renderClockFrameBuffered(hwnd win.HWND) {
	var ps win.PAINTSTRUCT
	hdc := win.BeginPaint(hwnd, &ps)
	defer win.EndPaint(hwnd, &ps)

	memDC := win.CreateCompatibleDC(hdc)
	if memDC == 0 {
		return
	}
	defer win.DeleteDC(memDC)

	memBitmap := win.CreateCompatibleBitmap(hdc, defaultClockWidth, defaultClockHeight)
	if memBitmap == 0 {
		return
	}
	defer win.DeleteObject(win.HGDIOBJ(memBitmap))

	oldBitmap := win.SelectObject(memDC, win.HGDIOBJ(memBitmap))
	defer win.SelectObject(memDC, oldBitmap)

	rect := win.RECT{
		Left:   0,
		Top:    0,
		Right:  defaultClockWidth,
		Bottom: defaultClockHeight,
	}

	winAPI.fillRect.Call(uintptr(memDC), uintptr(unsafe.Pointer(&rect)), uintptr(clockApp.brushHandle))

	oldFont := win.SelectObject(memDC, win.HGDIOBJ(clockApp.fontHandle))
	win.SetTextColor(memDC, clockApp.config.TextColor)
	win.SetBkMode(memDC, win.TRANSPARENT)

	timeUtf16Ptr := clockApp.timeBuffer.FormatNow(clockApp.config.TimeFormat)

	winAPI.drawText.Call(
		uintptr(memDC),
		uintptr(unsafe.Pointer(timeUtf16Ptr)),
		uintptr(5), // Строка всегда строго 5 символов: "HH:MM"
		uintptr(unsafe.Pointer(&rect)),
		dtLeft|dtNoClip,
	)
	win.SelectObject(memDC, oldFont)

	winAPI.bitBlt.Call(
		uintptr(hdc), 0, 0,
		uintptr(defaultClockWidth), uintptr(defaultClockHeight),
		uintptr(memDC), 0, 0,
		uintptr(srccopy),
	)
}

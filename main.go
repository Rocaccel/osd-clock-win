package main

import (
	_ "embed"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/getlantern/systray"
	"github.com/lxn/win"
	"github.com/pelletier/go-toml/v2"
)

//go:embed icon.ico
var iconBytes []byte

// ============================================================================
// 1. КОНСТАНТЫ И ДОМЕННЫЕ ТИПЫ
// ============================================================================

const (
	autoPosition = -1

	defaultClockWidth  = 400
	defaultClockHeight = 120
	screenMarginRight  = 30
	screenMarginBottom = 40

	lwaColorKey = 0x00000001
	dtLeft      = 0x00000000
	dtNoClip    = 0x00000100
	srccopy     = 0x00CC0020

	redrawIntervalMs  = 1000
	timerIDRender     = 1
	defaultColorName  = "green"
	defaultFontName   = "Consolas"
	defaultFontSize   = 48
	defaultFontWeight = 400
	defaultConfigFile = "config.toml"
)

type TimeFormat string

const (
	Format24H TimeFormat = "24h"
	Format12H TimeFormat = "12h"
)

type Config struct {
	FontName   string       `toml:"font_name"`
	FontSize   int          `toml:"font_size"`
	FontWeight int          `toml:"font_weight"`
	Color      string       `toml:"color"`
	TextColor  win.COLORREF `toml:"-"`
	PositionX  int          `toml:"pos_x"`
	PositionY  int          `toml:"pos_y"`
	TimeFormat TimeFormat   `toml:"time_format"`
}

type WindowBounds struct {
	X, Y          int32
	Width, Height int32
}

// TimeBuffer отвечает за формирование UTF-16 строки времени.
type TimeBuffer struct {
	utf16Buf [6]uint16
}

// Format заменяет группу методов FormatNow/FormatNow12h/FormatNow24h.
// Читаемость сигнатуры: t указывает на форматируемое время, tf — на формат.
// Имя метода сокращено до Format, так как имя типа TimeBuffer уже задает контекст.
func (tb *TimeBuffer) Format(t time.Time, tf TimeFormat) *uint16 {
	hour, min, _ := t.Clock()

	if tf == Format12H {
		hour = hour % 12
		if hour == 0 {
			hour = 12
		}
	}

	tb.utf16Buf[0] = uint16('0' + hour/10)
	tb.utf16Buf[1] = uint16('0' + hour%10)
	tb.utf16Buf[2] = uint16(':')
	tb.utf16Buf[3] = uint16('0' + min/10)
	tb.utf16Buf[4] = uint16('0' + min%10)
	tb.utf16Buf[5] = 0

	return &tb.utf16Buf[0]
}

// ============================================================================
// 2. ИНКАПСУЛЯЦИЯ WIN32 DLL
// ============================================================================

type win32API struct {
	setLayeredWindow *syscall.LazyProc
	createFont        *syscall.LazyProc
	drawText          *syscall.LazyProc
	fillRect          *syscall.LazyProc
	createSolidBrush *syscall.LazyProc
	bitBlt            *syscall.LazyProc
}

var (
	winAPIOnce sync.Once
	winAPI     *win32API
)

func getWin32API() *win32API {
	winAPIOnce.Do(func() {
		user32 := syscall.NewLazyDLL("user32.dll")
		gdi32 := syscall.NewLazyDLL("gdi32.dll")

		winAPI = &win32API{
			setLayeredWindow: user32.NewProc("SetLayeredWindowAttributes"),
			createFont:        gdi32.NewProc("CreateFontW"),
			drawText:          user32.NewProc("DrawTextW"),
			fillRect:          user32.NewProc("FillRect"),
			createSolidBrush: gdi32.NewProc("CreateSolidBrush"),
			bitBlt:            gdi32.NewProc("BitBlt"),
		}
	})
	return winAPI
}

// ============================================================================
// 3. ДОМЕННЫЙ ОБЪЕКТ ЧАСОВ И УПРАВЛЕНИЕ СОСТОЯНИЕМ
// ============================================================================

type ClockWindow struct {
	mu sync.RWMutex

	config Config
	hwnd   win.HWND

	screenWidth  int32
	screenHeight int32

	fontHandle  win.HFONT
	brushHandle win.HBRUSH

	timeBuffer TimeBuffer
	isVisible  bool
}

func NewClockWindow(cfg Config) *ClockWindow {
	cw := &ClockWindow{
		config:    cfg,
		isVisible: true,
	}
	cw.UpdateScreenMetrics()
	return cw
}

func (cw *ClockWindow) UpdateScreenMetrics() {
	sw := win.GetSystemMetrics(win.SM_CXSCREEN)
	sh := win.GetSystemMetrics(win.SM_CYSCREEN)

	cw.mu.Lock()
	cw.screenWidth = sw
	cw.screenHeight = sh
	cw.mu.Unlock()
}

func (cw *ClockWindow) CalculateBounds() WindowBounds {
	cw.mu.RLock()
	x := cw.config.PositionX
	y := cw.config.PositionY
	sw := cw.screenWidth
	sh := cw.screenHeight
	cw.mu.RUnlock()

	if x == autoPosition || y == autoPosition {
		if x == autoPosition {
			x = int(sw) - defaultClockWidth - screenMarginRight
		}
		if y == autoPosition {
			y = int(sh) - defaultClockHeight - screenMarginBottom
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
	cw.mu.RLock()
	hwnd := cw.hwnd
	cw.mu.RUnlock()

	if hwnd == 0 {
		return
	}
	bounds := cw.CalculateBounds()
	ok := win.SetWindowPos(
		hwnd,
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
	cw.mu.Lock()
	cw.isVisible = !cw.isVisible
	visible := cw.isVisible
	hwnd := cw.hwnd
	cw.mu.Unlock()

	if hwnd == 0 {
		return
	}

	if visible {
		win.ShowWindow(hwnd, win.SW_SHOW)
		menuItem.Check()
	} else {
		win.ShowWindow(hwnd, win.SW_HIDE)
		menuItem.Uncheck()
	}
}

// ReloadConfig отвечает за горячую перезагрузку параметров из config.toml.
// Процедурный стиль: чёткий последовательный поток действий без побочных эффектов CLI-флажков.
func (cw *ClockWindow) ReloadConfig() {
	configPath := findConfigFile()
	if configPath == "" {
		log.Printf("[WARN] Файл конфигурации не найден для перезагрузки")
		return
	}

	newCfg := defaultConfig()
	if fileData, err := os.ReadFile(configPath); err == nil {
		if err := toml.Unmarshal(fileData, &newCfg); err != nil {
			log.Printf("[WARN] Ошибка парсинга %s при перезагрузке: %v", configPath, err)
			return
		}
	}
	newCfg.TextColor = parseColor(newCfg.Color)

	cw.mu.Lock()
	cw.config = newCfg
	hwnd := cw.hwnd
	cw.mu.Unlock()

	// Инвалидируем старые GDI-ресурсы и пересоздаём их под новые параметры
	cw.FreeGDIResources()
	if err := cw.InitGDIResources(); err != nil {
		log.Printf("[ERROR] Сбой инициализации GDI-ресурсов: %v", err)
	}

	cw.UpdatePosition()
	if hwnd != 0 {
		win.InvalidateRect(hwnd, nil, false)
	}
	log.Printf("[INFO] Конфигурация успешно обновлена из %s", configPath)
}

func (cw *ClockWindow) InitGDIResources() error {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	api := getWin32API()

	fontNamePtr, err := syscall.UTF16PtrFromString(cw.config.FontName)
	if err != nil {
		log.Printf("[WARN] Некорректное имя шрифта '%s', сброс на дефолтный: %v", cw.config.FontName, err)
		fontNamePtr, _ = syscall.UTF16PtrFromString(defaultFontName)
	}

	hFont, _, _ := api.createFont.Call(
		uintptr(cw.config.FontSize), 0, 0, 0,
		uintptr(cw.config.FontWeight), 0, 0, 0,
		win.DEFAULT_CHARSET, 0, 0, 4, 0,
		uintptr(unsafe.Pointer(fontNamePtr)),
	)
	if hFont == 0 {
		return fmt.Errorf("ошибка вызова CreateFontW (code: %d)", win.GetLastError())
	}
	cw.fontHandle = win.HFONT(hFont)

	hBrush, _, _ := api.createSolidBrush.Call(uintptr(0))
	if hBrush == 0 {
		win.DeleteObject(win.HGDIOBJ(cw.fontHandle))
		cw.fontHandle = 0
		return fmt.Errorf("ошибка вызова CreateSolidBrush (code: %d)", win.GetLastError())
	}
	cw.brushHandle = win.HBRUSH(hBrush)

	return nil
}

func (cw *ClockWindow) FreeGDIResources() {
	cw.mu.Lock()
	defer cw.mu.Unlock()

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

func findConfigFile() string {
	if _, err := os.Stat(defaultConfigFile); err == nil {
		return defaultConfigFile
	}

	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		configNearExe := filepath.Join(exeDir, defaultConfigFile)
		if _, err := os.Stat(configNearExe); err == nil {
			return configNearExe
		}
	}

	return ""
}

// defaultConfig возвращает чистый базовый конфиг без побочных эффектов.
func defaultConfig() Config {
	return Config{
		FontName:   defaultFontName,
		FontSize:   defaultFontSize,
		FontWeight: defaultFontWeight,
		Color:      defaultColorName,
		PositionX:  autoPosition,
		PositionY:  autoPosition,
		TimeFormat: Format24H,
	}
}

// loadConfig выполняет только первичную инициализацию при старте приложения (с флагами CLI).
func loadConfig() Config {
	cfg := defaultConfig()

	configPath := findConfigFile()
	if configPath != "" {
		if fileData, err := os.ReadFile(configPath); err == nil {
			if err := toml.Unmarshal(fileData, &cfg); err != nil {
				log.Printf("[WARN] Ошибка парсинга %s: %v", configPath, err)
			}
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

var globalClockApp *ClockWindow
var atomicRenderState uint32

// ============================================================================
// 5. ТОЧКА ВХОДА И СИСТЕМНЫЙ ТРЕЙ
// ============================================================================

func main() {
	config := loadConfig()
	globalClockApp = NewClockWindow(config)

	systray.Run(onReady, onExit)
}

func onReady() {
	systray.SetIcon(iconBytes)
	systray.SetTitle("Desktop Clock")
	systray.SetTooltip("OSD-Clock")

	mToggleVisible := systray.AddMenuItem("Показать часы", "Показать/Спрятать")
	mToggleVisible.Check()
	systray.AddSeparator()
	mOpenConfig := systray.AddMenuItem("Открыть конфиг", "Редактировать конфиг")
	mReload := systray.AddMenuItem("Перезагрузить конфиг", "Перезагрузить конфиг")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Выход", "")

	go func() {
		for {
			select {
			case <-mToggleVisible.ClickedCh:
				globalClockApp.ToggleVisibility(mToggleVisible)
			case <-mQuit.ClickedCh:
				systray.Quit()
				return
			case <-mOpenConfig.ClickedCh:
				configPath := findConfigFile()
				if configPath == "" {
					configPath = defaultConfigFile
				}
				exec.Command("notepad.exe", configPath).Start()
			case <-mReload.ClickedCh:
				// Явный вызов метода перезагрузки
				globalClockApp.ReloadConfig()
			}
		}
	}()

	go startClockWindow()
}

func onExit() {
	if globalClockApp != nil {
		globalClockApp.mu.RLock()
		hwnd := globalClockApp.hwnd
		globalClockApp.mu.RUnlock()

		if hwnd != 0 {
			win.PostMessage(hwnd, win.WM_CLOSE, 0, 0)
		}
	}
}

// ============================================================================
// 6. WIN32 EVENT LOOP И ОТРИСОВКА КАДРА
// ============================================================================

func startClockWindow() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

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
		HCursor:        win.LoadCursor(0, win.MAKEINTRESOURCE(win.IDC_ARROW)),
	}

	if atom := win.RegisterClassEx(&wndClass); atom == 0 {
		log.Fatalf("[FATAL] Не удалось зарегистрировать класс окна. Код ошибки: %d", win.GetLastError())
	}

	bounds := globalClockApp.CalculateBounds()

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

	globalClockApp.mu.Lock()
	globalClockApp.hwnd = hwnd
	globalClockApp.mu.Unlock()

	if err := globalClockApp.InitGDIResources(); err != nil {
		log.Fatalf("[FATAL] Ошибка инициализации GDI ресурсов: %v", err)
	}

	globalClockApp.UpdatePosition()

	api := getWin32API()
	api.setLayeredWindow.Call(uintptr(hwnd), 0, 255, lwaColorKey)

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
		globalClockApp.UpdateScreenMetrics()
		globalClockApp.UpdatePosition()
		return 0

	case win.WM_TIMER:
		win.InvalidateRect(hwnd, nil, false)
		return 0

	case win.WM_ERASEBKGND:
		return 1

	case win.WM_PAINT:
		renderClockFrame(hwnd)
		return 0

	case win.WM_DESTROY:
		win.KillTimer(hwnd, timerIDRender)
		globalClockApp.FreeGDIResources()
		win.PostQuitMessage(0)
		return 0
	}

	return win.DefWindowProc(hwnd, msg, wParam, lParam)
}

// renderClockFrame переименована с renderClockFrameBuffered.
// Убраны лишние детали реализации из названия.
func renderClockFrame(hwnd win.HWND) {
	if !atomic.CompareAndSwapUint32(&atomicRenderState, 0, 1) {
		return
	}
	defer atomic.StoreUint32(&atomicRenderState, 0)

	var ps win.PAINTSTRUCT
	hdc := win.BeginPaint(hwnd, &ps)
	if hdc == 0 {
		return
	}
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

	globalClockApp.mu.RLock()
	brushHandle := globalClockApp.brushHandle
	fontHandle := globalClockApp.fontHandle
	textColor := globalClockApp.config.TextColor
	timeFormat := globalClockApp.config.TimeFormat
	globalClockApp.mu.RUnlock()

	api := getWin32API()

	api.fillRect.Call(uintptr(memDC), uintptr(unsafe.Pointer(&rect)), uintptr(brushHandle))

	oldFont := win.SelectObject(memDC, win.HGDIOBJ(fontHandle))
	win.SetTextColor(memDC, textColor)
	win.SetBkMode(memDC, win.TRANSPARENT)

	globalClockApp.mu.Lock()
	timeUtf16Ptr := globalClockApp.timeBuffer.Format(time.Now(), timeFormat)
	globalClockApp.mu.Unlock()

	api.drawText.Call(
		uintptr(memDC),
		uintptr(unsafe.Pointer(timeUtf16Ptr)),
		uintptr(5),
		uintptr(unsafe.Pointer(&rect)),
		dtLeft|dtNoClip,
	)

	win.SelectObject(memDC, oldFont)

	api.bitBlt.Call(
		uintptr(hdc), 0, 0,
		uintptr(defaultClockWidth), uintptr(defaultClockHeight),
		uintptr(memDC), 0, 0,
		uintptr(srccopy),
	)
}

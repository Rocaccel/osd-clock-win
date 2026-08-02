# OSD Clock for Windows

<p align="center"><a href="README.RU.MD">Русский</a> | <a href="README.MD">English</a></p>

---

A lightweight on-screen digital clock for Microsoft Windows written in Go using the native Win32 API.

The application displays a transparent clock above all windows, consumes virtually no system resources, and can be configured through a TOML configuration file or command-line arguments.

![Platform](https://img.shields.io/badge/platform-Windows-blue)
![Go](https://img.shields.io/badge/Go-1.24+-00ADD8)

---

## Features

- Native Win32 implementation (User32 + GDI)
- Transparent click-through window
- Always-on-top display
- System tray icon
- Automatic screen position adjustment
- 12-hour and 24-hour time formats
- TOML configuration support
- Command-line argument overrides
- No external GUI framework
- Extremely low CPU and memory usage

---

## Installation

Download the latest release or build from source.

### Build

```bash
go build -ldflags="-H windowsgui" -o osd-clock.exe
```

---

## Running

```
osd-clock.exe
```

The application starts minimized to the system tray and displays the clock on the desktop.

---

## Configuration

Configuration is stored in:

```
config.toml
```

Example:

```toml
font_name = "Ubuntu"
font_size = 50
font_weight = 600

color = "yellow"

pos_x = 1760
pos_y = 30

time_format = "12h"
```

### Configuration options

| Parameter | Description |
|-----------|-------------|
| `font_name` | Windows font name |
| `font_size` | Font size |
| `font_weight` | Font weight (100–900) |
| `color` | Clock text color |
| `pos_x` | Window X position (`-1` = automatic) |
| `pos_y` | Window Y position (`-1` = automatic) |
| `time_format` | `12h` or `24h` |

---

## Command-line options

Arguments override values from `config.toml`.

| Option | Description |
|---------|-------------|
| `-font` | Font name |
| `-size` | Font size |
| `-weight` | Font weight |
| `-color` | Text color |
| `-x` | X coordinate |
| `-y` | Y coordinate |
| `-format` | `12h` or `24h` |

Example:

```bash
osd-clock.exe -font "Segoe UI" -size 48 -color white
```

---

## Available colors

- green
- red
- blue
- white
- yellow
- cyan
- magenta
- gray

---

## System tray

The application places an icon in the Windows notification area.

Available actions:

- Hide/Show The Clock
- Exit application

---

## Architecture

The application is implemented using native Windows APIs:

- User32
- GDI32
- Layered windows
- Transparent click-through window
- Windows timers
- System tray notifications

No Electron, Qt, GTK, Wails, Fyne or other GUI frameworks are used.

---

## Performance

Designed for minimal overhead.

Characteristics:

- native Win32 rendering
- no GPU acceleration required
- timer refresh once per second
- zero-allocation time formatting
- negligible CPU usage

---

## Project structure

```
.
├── main.go
├── config.toml
├── icon.ico
├── LICENSE
├── go.mod
└── winres/
```

---

## Requirements

- Windows 10 or newer
- Go 1.24+

---

## Third-party libraries

- github.com/lxn/win
- github.com/getlantern/systray
- github.com/pelletier/go-toml/v2

---

## License

This project is distributed under the GLWT(Good Luck With That) Public License.

---

## Credits

Application icon:

Hour Glass icon by Fliqqer

https://www.flaticon.com/free-icons/hour-glass

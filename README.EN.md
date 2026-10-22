<div align="center">

<img src="winres/icon.png" width="96" alt="OSD Clock logo">

# OSD Clock for Windows

[![Platform](https://img.shields.io/badge/platform-Windows-0078D6?style=flat-square)](https://www.microsoft.com/windows)
[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev)
[![Scoop](https://img.shields.io/badge/Scoop-osd--clock-4DA94D?style=flat-square)](https://scoop.sh)
[![License](https://img.shields.io/badge/license-0BSD-blue?style=flat-square)](LICENSE)

[Russian](README.md) | English

[Features](#features) • [Installation](#installation) • [Usage](#usage) • [Configuration](#configuration) • [Building](#building)

</div>

A lightweight on-screen clock for Windows: a transparent window above all other windows, rendered with the native Win32 API (GDI), configured through `config.toml` or command-line flags. A single exe file, no third-party GUI frameworks, virtually no system load.

## Features

- Transparent window above all other windows, clicks pass through it
- System tray icon with menu — no hotkeys needed
- Hot reload of `config.toml` without restart, from the tray menu
- 12-hour and 24-hour time formats
- Choice of font, size, weight and text color
- Automatic positioning or exact pixel coordinates
- Automatic repositioning when the screen resolution changes
- Command-line flags override any config value

## Installation

### Via Scoop

```powershell
scoop bucket add rocaccel https://github.com/Rocaccel/scoop-bucket
scoop install osd-clock
```

The `config.toml` file survives updates (`persist` in the manifest), and the manifest is refreshed automatically on every release.

### From releases

Download and extract an archive from the [releases page](https://github.com/Rocaccel/osd-clock-win/releases), then run `osd-clock.exe`.

## Usage

Run `osd-clock.exe` — the clock appears in the bottom-right corner of the screen above all windows.

Command-line flags take priority over `config.toml`:

| Flag      | Description                            | Default    |
| --------- | -------------------------------------- | ---------- |
| `-color`  | Text color: `green`, `red`, `blue`, `white`, `yellow`, `cyan`, `magenta`, `gray` | `green` |
| `-size`   | Font size (10–200)                     | `48`       |
| `-font`   | Name of a font installed in the system | `Consolas` |
| `-weight` | Font weight (100–900)                  | `400`      |
| `-x`      | X coordinate, `-1` for auto            | `-1`       |
| `-y`      | Y coordinate, `-1` for auto            | `-1`       |
| `-format` | Time format: `24h` or `12h`            | `24h`      |

Example:

```powershell
.\osd-clock.exe -color cyan -size 64 -format 12h
```

### System tray

| Item                | Action                                                 |
| ------------------- | ------------------------------------------------------ |
| Show clock (Показать часы) | Hide or show the clock window                   |
| Open config (Открыть конфиг) | Open `config.toml` in Notepad                   |
| Reload config (Перезагрузить конфиг) | Apply `config.toml` changes without restart |
| Exit (Выход)        | Close the application                                  |

> [!NOTE]
> In the `12h` format no AM/PM suffix is shown: hours are simply converted to the 12-hour count.

## Configuration

The `config.toml` file is looked up first in the current directory, then next to `osd-clock.exe`. If no file is found, built-in defaults are used.

```toml
font_name = "Ubuntu"
font_size = 50
font_weight = 600
color = "yellow"
pos_x = 1760
pos_y = 30
time_format = "12h"
```

| Parameter     | Description                                            | Default     |
| ------------- | ------------------------------------------------------ | ----------- |
| `font_name`   | Name of a font installed in Windows                    | `Consolas`  |
| `font_size`   | Font size (10–200, 32–72 recommended)                  | `48`        |
| `font_weight` | Weight: `100` thin, `400` regular, `700` bold, etc.     | `400`       |
| `color`       | Text color from the palette (see flags table)          | `green`     |
| `pos_x`       | X coordinate in pixels, `-1` for auto                  | `-1`        |
| `pos_y`       | Y coordinate in pixels, `-1` for auto                  | `-1`        |
| `time_format` | `24h` or `12h`                                         | `24h`       |

Changes are applied via the "Reload config" tray item — no restart needed.

> [!TIP]
> Priority is simple: defaults → `config.toml` → command-line flags.

## Building

Requires [Go 1.25](https://go.dev/dl/) and Windows to build (Win32 API is used).

```powershell
git clone https://github.com/Rocaccel/osd-clock-win
cd osd-clock-win
go build -ldflags "-H windowsgui" -o osd-clock.exe
```

The `-H windowsgui` flag suppresses the console window. The icon is embedded from `rsrc.syso` (generated from `winres/winres.json` with [go-winres](https://github.com/tc-hib/go-winres)).

## Project structure

```
osd-clock-win/
├── main.go            # entire application code
├── config.toml        # example configuration
├── icon.ico           # icon for tray and rsrc.syso
├── rsrc.syso          # embedded icon (build)
├── winres/            # icon sources and winres.json
├── .github/           # scoop manifest publishing workflow
├── go.mod
└── go.sum
```

## Libraries

- [lxn/win](https://github.com/lxn/win) — Win32 API bindings
- [getlantern/systray](https://github.com/getlantern/systray) — tray icon and menu
- [pelletier/go-toml](https://github.com/pelletier/go-toml) — TOML parser

## Acknowledgments

Thanks to [lxn/win](https://github.com/lxn/win) for making pure Win32 on Go possible without cgo madness.

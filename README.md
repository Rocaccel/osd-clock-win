<div align="center">

<img src="winres/icon.png" width="96" alt="OSD Clock logo">

# OSD Clock для Windows

[![Платформа](https://img.shields.io/badge/platform-Windows-0078D6?style=flat-square)](https://www.microsoft.com/windows)
[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev)
[![Scoop](https://img.shields.io/badge/Scoop-osd--clock-4DA94D?style=flat-square)](https://scoop.sh)
[![Лицензия](https://img.shields.io/badge/license-0BSD-blue?style=flat-square)](LICENSE)

Русский | [English](README.EN.md)

[Возможности](#возможности) • [Установка](#установка) • [Использование](#использование) • [Конфигурация](#конфигурация) • [Сборка](#сборка)

</div>

Лёгкие экранные часы для Windows: прозрачное окно поверх всех окон, отрисовка нативным Win32 API (GDI), настройка через `config.toml` или флаги командной строки. Один exe-файл, без сторонних GUI-фреймворков, практически без нагрузки на систему.

## Возможности

- Прозрачное окно поверх всех окон, клики сквозь него проходят мимо
- Значок в системном трее с управлением без горячих клавиш
- Горячая перезагрузка `config.toml` без перезапуска — из меню трей
- 12- и 24-часовой формат времени
- Выбор шрифта, размера, жирности и цвета текста
- Автоматическое позиционирование или точные координаты
- Автоподстройка при смене разрешения экрана
- Флаги командной строки переопределяют любой параметр из конфига

## Установка

### Через Scoop

```powershell
scoop bucket add rocaccel https://github.com/Rocaccel/scoop-bucket
scoop install osd-clock
```

Конфигурация из `config.toml` переживает обновления (`persist` в манифесте), а манифест автоматически обновляется при каждой публикации релиза.

### Из релизов

Скачайте и распакуйте архив из [релизов](https://github.com/Rocaccel/osd-clock-win/releases), затем запустите `osd-clock.exe`.

## Использование

Запустите `osd-clock.exe` — часы появятся в правом нижнем углу экрана поверх всех окон.

Флаги командной строки имеют приоритет над `config.toml`:

| Флаг      | Описание                              | По умолчанию |
| --------- | ------------------------------------- | ------------ |
| `-color`  | Цвет текста: `green`, `red`, `blue`, `white`, `yellow`, `cyan`, `magenta`, `gray` | `green` |
| `-size`   | Размер шрифта (10–200)                | `48`         |
| `-font`   | Название шрифта из системы            | `Consolas`   |
| `-weight` | Жирность шрифта (100–900)             | `400`        |
| `-x`      | Координата X, `-1` — авто             | `-1`         |
| `-y`      | Координата Y, `-1` — авто             | `-1`         |
| `-format` | Формат времени: `24h` или `12h`        | `24h`        |

Пример:

```powershell
.\osd-clock.exe -color cyan -size 64 -format 12h
```

### Системный трей

| Пункт                 | Действие                                              |
| --------------------- | ----------------------------------------------------- |
| Показать часы         | Скрыть или показать окно часов                        |
| Открыть конфиг        | Открыть `config.toml` в Блокноте                      |
| Перезагрузить конфиг  | Применить изменения из `config.toml` без перезапуска  |
| Выход                 | Закрыть приложение                                   |

> [!NOTE]
> В формате `12h` AM/PM не отображается: часы просто переводятся в 12-часовой отсчёт.

## Конфигурация

Файл `config.toml` ищется сначала в текущей директории, затем рядом с `osd-clock.exe`. Если файла нет, используются значения по умолчанию.

```toml
font_name = "Ubuntu"
font_size = 50
font_weight = 600
color = "yellow"
pos_x = 1760
pos_y = 30
time_format = "12h"
```

| Параметр      | Описание                                             | По умолчанию |
| ------------- | ---------------------------------------------------- | ------------ |
| `font_name`   | Имя шрифта, установленного в Windows                 | `Consolas`   |
| `font_size`   | Размер шрифта (10–200, рекомендуется 32–72)          | `48`         |
| `font_weight` | Жирность: `100` thin, `400` regular, `700` bold и т.д. | `400`      |
| `color`       | Цвет текста из палитры (см. таблицу флагов)          | `green`      |
| `pos_x`       | Координата X в пикселях, `-1` — авто                 | `-1`         |
| `pos_y`       | Координата Y в пикселях, `-1` — авто                 | `-1`         |
| `time_format` | `24h` или `12h`                                       | `24h`        |

Изменения применяются через пункт «Перезагрузить конфиг» в меню трей — перезапуск не нужен.

> [!TIP]
> С приоритетами просто: дефолты → `config.toml` → флаги командной строки.

## Сборка

Требуется [Go 1.25](https://go.dev/dl/) и Windows для сборки (используются Win32 API).

```powershell
git clone https://github.com/Rocaccel/osd-clock-win
cd osd-clock-win
go build -ldflags "-H windowsgui" -o osd-clock.exe
```

Флаг `-H windowsgui` убирает консольное окно. Иконка подставляется из `rsrc.syso` (генерируется из `winres/winres.json` через [go-winres](https://github.com/tc-hib/go-winres)).

## Структура проекта

```
osd-clock-win/
├── main.go            # весь код приложения
├── config.toml        # пример конфигурации
├── icon.ico           # иконка для трей и rsrc.syso
├── rsrc.syso          # встроенная иконка (сборка)
├── winres/            # исходники иконки и winres.json
├── .github/           # workflow публикации scoop-манифеста
├── go.mod
└── go.sum
```

## Библиотеки

- [lxn/win](https://github.com/lxn/win) — биндинги Win32 API
- [getlantern/systray](https://github.com/getlantern/systray) — значок и меню в трее
- [pelletier/go-toml](https://github.com/pelletier/go-toml) — парсер TOML

## Благодарности

Спасибо [lxn/win](https://github.com/lxn/win) за возможность писать чистый Win32 на Go без cgo-кошмаров.

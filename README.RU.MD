# OSD Clock для Windows

<p align="center"><a href="README.RU.MD">Русский</a> | <a href="README.MD">English</a></p>

---

Легкие цифровые часы для Microsoft Windows, написанные на Go с использованием нативного Win32 API.

Приложение отображает прозрачные часы поверх рабочего стола, практически не потребляет системные ресурсы и поддерживает настройку через файл `config.toml` или параметры командной строки.

![Platform](https://img.shields.io/badge/Платформа-Windows-blue)
![Go](https://img.shields.io/badge/Go-1.24+-00ADD8)

---

## Возможности

- Нативное Win32-приложение
- Прозрачное окно поверх рабочего стола
- Сквозные клики мышью (click-through)
- Всегда поверх остальных окон
- Значок в системном трее
- Автоматическое позиционирование
- Поддержка 12- и 24-часового формата времени
- Настройка через `config.toml`
- Переопределение настроек параметрами командной строки
- Не использует сторонние GUI-фреймворки
- Минимальное потребление памяти и процессорного времени

---

## Установка

Скачайте готовую сборку [![Latest Release](https://img.shields.io/github/v/release/Rocaccel/osd-clock-win?label=Latest%20Release)](https://github.com/Rocaccel/osd-clock-win/releases/latest) или соберите программу самостоятельно.

### Установка через Scoop

Программа может быть установлена через встроенный Scoop bucket.

#### Добавить bucket

```powershell
scoop bucket add osd-clock-win https://github.com/Rocaccel/osd-clock-win
```

#### Установить программу

```powershell
scoop install osd-clock
```

#### Обновить программу

```powershell
scoop update osd-clock
```

#### Обновить все установленные приложения

```powershell
scoop update *
```


### Сборка

```bash
go build -ldflags="-H windowsgui" -o osd-clock.exe
```

---

## Запуск

```
osd-clock.exe
```

После запуска приложение помещает значок в системный трей и отображает часы на рабочем столе.

---

## Конфигурация

Настройки находятся в файле

```
config.toml
```

Пример:

```toml
font_name = "Ubuntu"
font_size = 50
font_weight = 600

color = "yellow"

pos_x = 1760
pos_y = 30

time_format = "12h"
```

### Параметры

| Параметр | Описание |
|----------|----------|
| `font_name` | Название шрифта Windows |
| `font_size` | Размер шрифта |
| `font_weight` | Толщина шрифта (100–900) |
| `color` | Цвет текста |
| `pos_x` | Координата X (`-1` — автоматически) |
| `pos_y` | Координата Y (`-1` — автоматически) |
| `time_format` | `12h` или `24h` |

---

## Параметры командной строки

Аргументы командной строки имеют приоритет над настройками из `config.toml`.

| Параметр | Описание |
|----------|----------|
| `-font` | Шрифт |
| `-size` | Размер |
| `-weight` | Толщина |
| `-color` | Цвет |
| `-x` | Координата X |
| `-y` | Координата Y |
| `-format` | Формат времени (`12h` или `24h`) |

Пример:

```bash
osd-clock.exe -font "Segoe UI" -size 48 -color white
```

---

## Доступные цвета

- green
- red
- blue
- white
- yellow
- cyan
- magenta
- gray

---

## Системный трей

После запуска приложение добавляет значок в область уведомлений Windows.

Доступное действие:

- Выход из программы

---

## Особенности реализации

Проект полностью написан на Go с использованием нативных Windows API.

Используются:

- User32
- GDI32
- Layered Windows
- Transparent Click-through Window
- Windows Timer
- System Tray API

Программа **не использует** Electron, Qt, GTK, Wails, Fyne и другие графические фреймворки.

---

## Производительность

Проект ориентирован на максимально низкое потребление ресурсов.

Особенности реализации:

- нативный Win32 API;
- обновление один раз в секунду;
- отсутствие лишних выделений памяти при форматировании времени;
- очень низкая нагрузка на процессор;
- минимальное потребление оперативной памяти.

---

## Структура проекта

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

## Требования

- Windows 10 или новее
- Go 1.24+

---

## Используемые библиотеки

- github.com/lxn/win
- github.com/getlantern/systray
- github.com/pelletier/go-toml/v2

---

## Лицензия

Проект распространяется по лицензии GLWT(Good Luck With That) Public License.

---

## Благодарности

Иконка приложения:

Hour Glass icon by Fliqqer

https://www.flaticon.com/free-icons/hour-glass

# SwingWinGo / Swinging Windows

**SwingWinGo** is a fun Windows utility that lets you swing desktop windows like a “swing” (horizontal, vertical, circle, ellipse, figure-eight). It's said to help reduce visual fatigue from long periods of screen use, and you can optionally move the cursor along with the target window.

> This project is a Go reimplementation inspired by [Central Fixation](https://www.central-fixation.com/). Since the original download link is no longer available, this project is provided as open source.

> This is an English translation of the original `README_zh.md`. The canonical Chinese version is maintained in `README_zh.md`.

---

## 🛠 System Requirements

- **OS**: Windows 10 / 11 (tested on Windows 11 25H2)

## 🔥 Key Features

- **Window list**: Automatically lists all visible windows on the desktop.
- **Multi-window swinging**: Supports swinging multiple windows at once.
- **Swing modes**: Horizontal, vertical, circle, ellipse, and figure-eight trajectories.
- **Custom parameters**: Adjust swing speed and horizontal/vertical amplitude.
- **Cursor follow**: Optional “move cursor while hovering over target window” mode.

## ⚙ Usage

1. Download the compiled `SwingWinGo.exe` and place it in any folder.
2. Run the program and click `Refresh` to load the current window list.
3. Check the windows you want to swing.
4. Choose a swing mode and set a comfortable speed/amplitude. Start with lower speed/amplitude to avoid discomfort.
5. Click `Start Swinging` to begin; click `Stop Swinging` to stop.

---

## 🛠 Build from Source

If you want to build from source:

1. Clone the repo

```powershell
git clone <repo-url>
cd <repo-folder>
```

2. Build

```powershell
go build -buildvcs=false -ldflags "-H windowsgui -s -w" -o SwingWinGo.exe .
```

---

## 🧩 TODO

- **Background/minimized mode**: Run without a visible GUI or minimize to the system tray.
- **Save settings**: Remember selected target windows and auto-load them on startup.
- **Improve internationalization**: Fix Chinese garbled text (current version does not support displaying Chinese window titles).

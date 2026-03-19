//go:build windows

package ui

import (
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"swingwingo/swinger"
)

func Run() {
	a := app.New()
	a.Settings().SetTheme(theme.DarkTheme())
	w := a.NewWindow("SwingWinGo")
	w.Resize(fyne.NewSize(480, 620))

	engine := swinger.NewEngine()

	// ── Window checklist ──────────────────────────────────────────
	var cachedWindows []swinger.WindowInfo
	var checkboxes []*widget.Check
	windowListBox := container.NewVBox()

	rebuildList := func() {
		cachedWindows = swinger.EnumVisibleWindows()
		checkboxes = make([]*widget.Check, len(cachedWindows))
		windowListBox.RemoveAll()
		for i, wi := range cachedWindows {
			proc := wi.ProcessName
			if len(proc) > 4 && strings.ToLower(proc[len(proc)-4:]) == ".exe" {
				proc = proc[:len(proc)-4]
			}

			titleRunes := []rune(wi.Title)
			title := wi.Title
			if len(titleRunes) > 40 {
				title = string(titleRunes[:40]) + "..."
			}

			label := fmt.Sprintf("%s  ›  %s  [%d×%d]", proc, title, wi.Width, wi.Height)
			cb := widget.NewCheck(label, nil)
			checkboxes[i] = cb
			windowListBox.Add(cb)
		}
		windowListBox.Refresh()
	}

	refreshBtn := widget.NewButtonWithIcon("Refresh", theme.ViewRefreshIcon(), rebuildList)
	windowScroll := container.NewVScroll(windowListBox)
	windowScroll.SetMinSize(fyne.NewSize(460, 180))

	// ── Swing mode ────────────────────────────────────────────────
	modeOptions := []string{
		"Horizontal (Left-Right)",
		"Vertical (Up-Down)",
		"Circle",
		"Ellipse",
		"Figure-Eight",
	}
	modeSelect := widget.NewSelect(modeOptions, nil)
	modeSelect.SetSelected("Horizontal (Left-Right)")

	// ── Speed ─────────────────────────────────────────────────────
	speedLabel := widget.NewLabel("Speed: 0.50 cycles/sec")
	speedSlider := widget.NewSlider(0.05, 3.0)
	speedSlider.Step = 0.05
	speedSlider.Value = 0.5

	// ── Amplitude X ───────────────────────────────────────────────
	ampXLabel := widget.NewLabel("Amplitude (H): 20 px")
	ampXSlider := widget.NewSlider(1, 150)
	ampXSlider.Step = 1
	ampXSlider.Value = 20

	// ── Amplitude Y (Ellipse / Figure-Eight only) ─────────────────
	ampYLabel := widget.NewLabel("Amplitude (V): 20 px")
	ampYSlider := widget.NewSlider(1, 150)
	ampYSlider.Step = 1
	ampYSlider.Value = 20
	ampYBox := container.NewVBox(ampYLabel, ampYSlider)
	ampYBox.Hide()

	// ── Mouse swing ───────────────────────────────────────────────
	mouseCheck := widget.NewCheck("Mouse swings when over window", nil)

	// ── Status ────────────────────────────────────────────────────
	statusLabel := widget.NewLabel("Status: Stopped")
	statusLabel.Alignment = fyne.TextAlignCenter

	// ── Start / Stop ──────────────────────────────────────────────
	startBtn := widget.NewButton("Start Swinging", nil)
	startBtn.Importance = widget.HighImportance

	liveUpdate := func() {
		if !engine.IsRunning() {
			return
		}
		cfg := engine.GetConfig()
		cfg.Speed = speedSlider.Value
		cfg.Amplitude = ampXSlider.Value
		cfg.AmplitudeY = ampYSlider.Value
		cfg.Mode = modeFromString(modeSelect.Selected)
		cfg.MoveMouse = mouseCheck.Checked
		engine.SetConfig(cfg)
	}

	speedSlider.OnChanged = func(v float64) {
		speedLabel.SetText(fmt.Sprintf("Speed: %.2f cycles/sec", v))
		liveUpdate()
	}
	ampXSlider.OnChanged = func(v float64) {
		ampXLabel.SetText(fmt.Sprintf("Amplitude (H): %d px", int(v)))
		liveUpdate()
	}
	ampYSlider.OnChanged = func(v float64) {
		ampYLabel.SetText(fmt.Sprintf("Amplitude (V): %d px", int(v)))
		liveUpdate()
	}
	modeSelect.OnChanged = func(s string) {
		if s == "Ellipse" || s == "Figure-Eight" {
			ampYBox.Show()
		} else {
			ampYBox.Hide()
		}
		liveUpdate()
	}
	mouseCheck.OnChanged = func(_ bool) { liveUpdate() }

	startBtn.OnTapped = func() {
		if engine.IsRunning() {
			engine.Stop()
			startBtn.SetText("Start Swinging")
			startBtn.Importance = widget.HighImportance
			statusLabel.SetText("Status: Stopped")
			startBtn.Refresh()
			return
		}

		var targets []swinger.WindowInfo
		for i, cb := range checkboxes {
			if cb.Checked && i < len(cachedWindows) {
				targets = append(targets, cachedWindows[i])
			}
		}
		if len(targets) == 0 {
			statusLabel.SetText("Please click Refresh, then check at least one window!")
			return
		}

		cfg := swinger.Config{
			Mode:       modeFromString(modeSelect.Selected),
			Speed:      speedSlider.Value,
			Amplitude:  ampXSlider.Value,
			AmplitudeY: ampYSlider.Value,
			MoveMouse:  mouseCheck.Checked,
			Targets:    targets,
		}
		engine.SetConfig(cfg)

		if err := engine.Start(); err != nil {
			statusLabel.SetText("Error: " + err.Error())
			return
		}

		startBtn.SetText("Stop Swinging")
		startBtn.Importance = widget.DangerImportance
		statusLabel.SetText(fmt.Sprintf("Swinging %d window(s) | %s | %s px | %.2f Hz",
			len(targets),
			modeSelect.Selected,
			ampDisplay(modeFromString(modeSelect.Selected), ampXSlider.Value, ampYSlider.Value),
			speedSlider.Value,
		))
		startBtn.Refresh()
	}

	presetSoft := widget.NewButton("Gentle", func() {
		speedSlider.SetValue(0.3)
		ampXSlider.SetValue(10)
		ampYSlider.SetValue(10)
		modeSelect.SetSelected("Horizontal (Left-Right)")
	})
	presetMed := widget.NewButton("Normal", func() {
		speedSlider.SetValue(0.5)
		ampXSlider.SetValue(20)
		ampYSlider.SetValue(20)
		modeSelect.SetSelected("Horizontal (Left-Right)")
	})
	presetStrong := widget.NewButton("Strong", func() {
		speedSlider.SetValue(1.0)
		ampXSlider.SetValue(50)
		ampYSlider.SetValue(50)
		modeSelect.SetSelected("Circle")
	})
	presets := container.NewGridWithColumns(3, presetSoft, presetMed, presetStrong)

	content := container.NewVBox(
		widget.NewCard("Target Windows", "",
			container.NewVBox(
				container.NewBorder(nil, nil, nil, refreshBtn, widget.NewLabel("Check windows to swing:")),
				windowScroll,
			),
		),
		widget.NewCard("Swing Mode", "", modeSelect),
		widget.NewCard("Parameters", "", container.NewVBox(
			speedLabel, speedSlider,
			ampXLabel, ampXSlider,
			ampYBox,
			mouseCheck,
		)),
		widget.NewCard("Presets", "", presets),
		statusLabel,
		startBtn,
	)

	// 自動在啟動時執行一次重整，提升 UX
	rebuildList()

	w.SetContent(container.NewPadded(content))
	w.ShowAndRun()
}

func modeFromString(s string) swinger.SwingMode {
	switch s {
	case "Vertical (Up-Down)":
		return swinger.ModeVertical
	case "Circle":
		return swinger.ModeCircle
	case "Ellipse":
		return swinger.ModeEllipse
	case "Figure-Eight":
		return swinger.ModeFigureEight
	default:
		return swinger.ModeHorizontal
	}
}

func ampDisplay(mode swinger.SwingMode, ax, ay float64) string {
	if mode == swinger.ModeEllipse || mode == swinger.ModeFigureEight {
		return strconv.Itoa(int(ax)) + "x" + strconv.Itoa(int(ay))
	}
	return strconv.Itoa(int(ax))
}

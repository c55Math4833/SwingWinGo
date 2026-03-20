//go:build windows

package swinger

import (
	"math"
	"sync"
	"time"
)

// SwingMode defines the type of window oscillation
type SwingMode int

const (
	ModeHorizontal SwingMode = iota
	ModeVertical
	ModeCircle
	ModeEllipse
	ModeFigureEight
)

// Config holds all swing parameters
type Config struct {
	Mode       SwingMode
	Speed      float64 // cycles per second
	Amplitude  float64 // horizontal pixels
	AmplitudeY float64 // vertical pixels (Ellipse / Figure-Eight)
	MoveMouse  bool
	Targets    []WindowInfo // windows to swing
}

// windowState tracks per-window origin captured at Start
type windowState struct {
	hwnd                uintptr
	originX, originY    int32
	winWidth, winHeight int32
	currX, currY        float64 // 精確浮點位置，用於累積小量平滑
	velX, velY          float64 // 慣性速度，用於彈簧阻尼平滑
	lastX, lastY        int32   // 用於避免同一像素重複更新
	wasMinimized        bool
	wasFullScreen       bool
}

// Engine controls the swing loop
type Engine struct {
	mu      sync.Mutex
	cfg     Config
	running bool
	stopCh  chan struct{}
	windows []windowState
}

func NewEngine() *Engine { return &Engine{} }

func (e *Engine) SetConfig(cfg Config) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cfg = cfg
}

func (e *Engine) GetConfig() Config {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.cfg
}

func (e *Engine) IsRunning() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.running
}

func (e *Engine) Start() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		return nil
	}

	// Snapshot origin positions for every target window
	states := make([]windowState, 0, len(e.cfg.Targets))
	for _, wi := range e.cfg.Targets {
		rect, err := GetWindowRect(wi.HWND)
		if err != nil {
			continue
		}
		states = append(states, windowState{
			hwnd:          wi.HWND,
			originX:       rect.Left,
			originY:       rect.Top,
			winWidth:      rect.Right - rect.Left,
			winHeight:     rect.Bottom - rect.Top,
			currX:         float64(rect.Left),
			currY:         float64(rect.Top),
			lastX:         rect.Left,
			lastY:         rect.Top,
			wasMinimized:  IsWindowMinimized(wi.HWND),
			wasFullScreen: IsWindowFullScreen(wi.HWND),
		})
	}
	if len(states) == 0 {
		return nil
	}

	e.windows = states
	e.running = true
	e.stopCh = make(chan struct{})
	go e.loop()
	return nil
}

func (e *Engine) Stop() {
	e.mu.Lock()
	if !e.running {
		e.mu.Unlock()
		return
	}
	stopCh := e.stopCh
	wins := e.windows
	e.mu.Unlock()

	close(stopCh)
	time.Sleep(50 * time.Millisecond)

	// Restore all windows to their origin positions
	for _, ws := range wins {
		SetWindowPos(ws.hwnd, ws.originX, ws.originY)
	}
}

func (e *Engine) loop() {
	defer func() {
		e.mu.Lock()
		e.running = false
		e.mu.Unlock()
	}()

	const fps = 60
	ticker := time.NewTicker(time.Second / fps)
	defer ticker.Stop()

	startTime := time.Now()
	lastTime := startTime
	var paused bool
	var pauseStart time.Time

	// 記錄前一幀的「整數像素偏移量」，以確保滑鼠動量計算精確
	var prevOffsetX, prevOffsetY int32

	for {
		select {
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.mu.Lock()
			cfg := e.cfg
			wins := e.windows
			e.mu.Unlock()

			now := time.Now()
			dt := now.Sub(lastTime).Seconds()
			lastTime = now

			// 檢查滑鼠左右鍵是否被按下
			isPressed := IsMouseButtonPressed()

			if isPressed {
				if !paused {
					pauseStart = now
					paused = true
				}
				// 暫停時間流逝與視窗位移
				continue
			}

			if paused {
				// 解除暫停：修正時鐘起點，跳過停頓間隔，避免位置突跳
				startTime = startTime.Add(now.Sub(pauseStart))
				paused = false

				// 剛放開滑鼠：使用者可能在暫停期間手動拖曳了視窗
				// 重新校準初始位置 (Origin)，避免視窗瞬移回原位
				for i, ws := range wins {
					rect, err := GetWindowRect(ws.hwnd)
					if err == nil {
						wins[i].originX = rect.Left - prevOffsetX
						wins[i].originY = rect.Top - prevOffsetY
						wins[i].currX = float64(rect.Left)
						wins[i].currY = float64(rect.Top)
						wins[i].velX = 0
						wins[i].velY = 0
					}
				}

				// 寫回引擎狀態
				e.mu.Lock()
				for i := range e.windows {
					e.windows[i].originX = wins[i].originX
					e.windows[i].originY = wins[i].originY
					e.windows[i].currX = wins[i].currX
					e.windows[i].currY = wins[i].currY
					e.windows[i].velX = wins[i].velX
					e.windows[i].velY = wins[i].velY
					e.windows[i].lastX = wins[i].lastX
					e.windows[i].lastY = wins[i].lastY
				}
				e.mu.Unlock()
			}

			// 在每一個走訪循環中直接計算理論目標位置，避免 elapsed 積累誤差
			elapsed := now.Sub(startTime).Seconds()
			dx, dy := computeOffset(cfg, elapsed)

			// 計算當前幀的整數像素偏移量
			currOffsetX := int32(math.Round(dx))
			currOffsetY := int32(math.Round(dy))

			// 彈簧阻尼追蹤目標：加強平滑並維持軌跡形狀
			const stiffness = 45.0
			const damping = 2.0 * 6.708203932499369 // approx 2*sqrt(45)
			deltas := make(map[uintptr]struct{ dx, dy int32 }, len(wins))
			for i := range wins {
				ws := &wins[i]

				currentMinimized := IsWindowMinimized(ws.hwnd)
				currentFullScreen := IsWindowFullScreen(ws.hwnd)

				if currentMinimized || currentFullScreen {
					// 禁止搖擺被最小化或全螢幕的視窗
					ws.wasMinimized = currentMinimized
					ws.wasFullScreen = currentFullScreen
					deltas[ws.hwnd] = struct{ dx, dy int32 }{0, 0}
					continue
				}

				if ws.wasMinimized || ws.wasFullScreen {
					// 窗口恢復正常尺寸時重新同步位置基準
					rect, err := GetWindowRect(ws.hwnd)
					if err == nil {
						ws.originX = rect.Left - currOffsetX
						ws.originY = rect.Top - currOffsetY
						ws.currX = float64(rect.Left)
						ws.currY = float64(rect.Top)
						ws.velX = 0
						ws.velY = 0
						ws.lastX = rect.Left
						ws.lastY = rect.Top
					}
					ws.wasMinimized = false
					ws.wasFullScreen = false
					continue
				}

				oldX, oldY := ws.lastX, ws.lastY
				targetX := float64(ws.originX) + dx
				targetY := float64(ws.originY) + dy

				accX := stiffness*(targetX-ws.currX) - damping*ws.velX
				accY := stiffness*(targetY-ws.currY) - damping*ws.velY
				ws.velX += accX * dt
				ws.velY += accY * dt
				ws.currX += ws.velX * dt
				ws.currY += ws.velY * dt

				newX := int32(math.Round(ws.currX))
				newY := int32(math.Round(ws.currY))

				if newX != oldX || newY != oldY {
					SetWindowPos(ws.hwnd, newX, newY)
				}

				ws.lastX = newX
				ws.lastY = newY
				deltas[ws.hwnd] = struct{ dx, dy int32 }{newX - oldX, newY - oldY}
			}

			// 滑鼠動量疊加邏輯
			if cfg.MoveMouse {
				pt, err := GetCursorPos()
				if err == nil {
					// 1. 取得目前滑鼠精確指著的視窗 (可判斷遮擋)
					hoverWnd := WindowFromPoint(pt.X, pt.Y)
					if hoverWnd != 0 {
						// 2. 因為指到的可能是按鈕等子元件，溯源到最頂層主視窗
						rootWnd := GetAncestor(hoverWnd, GA_ROOT)

						// 3. 檢查這個頂層視窗是否是我們正在搖擺的目標
						if delta, ok := deltas[rootWnd]; ok {
							if delta.dx != 0 || delta.dy != 0 {
								SetCursorPos(pt.X+delta.dx, pt.Y+delta.dy)
							}
						}
					}
				}
			}

			prevOffsetX = currOffsetX
			prevOffsetY = currOffsetY
		}
	}
}

func triangleWave(x float64) float64 {
	// 轉換為振幅 [-1,1] 的三角波，維持固定速度折返
	return (2.0 / math.Pi) * math.Asin(math.Sin(x))
}

func computeOffset(cfg Config, t float64) (float64, float64) {
	omega := 2 * math.Pi * cfg.Speed
	phase := omega * t
	amp := cfg.Amplitude
	ampY := cfg.AmplitudeY
	if ampY == 0 {
		ampY = amp
	}
	switch cfg.Mode {
	case ModeHorizontal:
		return amp * triangleWave(phase), 0
	case ModeVertical:
		return 0, ampY * triangleWave(phase)
	case ModeCircle:
		return amp * math.Sin(phase), amp * (math.Cos(phase) - 1)
	case ModeEllipse:
		return amp * math.Sin(phase), ampY * (math.Cos(phase) - 1)
	case ModeFigureEight:
		return amp * math.Sin(phase), ampY * math.Sin(2*phase) / 2
	}
	return 0, 0
}

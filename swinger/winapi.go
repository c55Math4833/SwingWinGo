//go:build windows

package swinger

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32                       = windows.NewLazySystemDLL("user32.dll")
	procGetForegroundWindow      = user32.NewProc("GetForegroundWindow")
	procGetWindowRect            = user32.NewProc("GetWindowRect")
	procSetWindowPos             = user32.NewProc("SetWindowPos")
	procGetWindowTextW           = user32.NewProc("GetWindowTextW")
	procEnumWindows              = user32.NewProc("EnumWindows")
	procIsWindowVisible          = user32.NewProc("IsWindowVisible")
	procGetCursorPos             = user32.NewProc("GetCursorPos")
	procSetCursorPos             = user32.NewProc("SetCursorPos")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procGetAsyncKeyState         = user32.NewProc("GetAsyncKeyState")

	// 新增用於碰撞與遮擋偵測的 API
	procWindowFromPoint = user32.NewProc("WindowFromPoint")
	procGetAncestor     = user32.NewProc("GetAncestor")

	kernel32                       = windows.NewLazySystemDLL("kernel32.dll")
	procOpenProcess                = kernel32.NewProc("OpenProcess")
	procCloseHandle                = kernel32.NewProc("CloseHandle")
	procQueryFullProcessImageNameW = kernel32.NewProc("QueryFullProcessImageNameW")
)

const (
	PROCESS_QUERY_LIMITED_INFORMATION = 0x1000
)

const (
	SWP_NOSIZE         = 0x0001
	SWP_NOZORDER       = 0x0004
	SWP_NOACTIVATE     = 0x0010
	SWP_NOSENDCHANGING = 0x0400
)

// GA_ROOT retrieves the root window by walking the chain of parent windows.
const GA_ROOT = 2

// selfPID is this process's PID; its windows are excluded from enumeration
var selfPID = uint32(windows.GetCurrentProcessId())

// getWindowPID returns the PID of the process owning the given window
func getWindowPID(hwnd uintptr) uint32 {
	var pid uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	return pid
}

// getProcessName returns just the exe filename (e.g. "chrome.exe") for a PID
func getProcessName(pid uint32) string {
	handle, _, _ := procOpenProcess.Call(
		PROCESS_QUERY_LIMITED_INFORMATION, 0, uintptr(pid))
	if handle == 0 {
		return ""
	}
	defer procCloseHandle.Call(handle)

	buf := make([]uint16, 260)
	size := uint32(len(buf))
	ret, _, _ := procQueryFullProcessImageNameW.Call(
		handle, 0,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)))
	if ret == 0 {
		return ""
	}
	full := syscall.UTF16ToString(buf[:size])
	for i := len(full) - 1; i >= 0; i-- {
		if full[i] == '\\' || full[i] == '/' {
			return full[i+1:]
		}
	}
	return full
}

type RECT struct {
	Left, Top, Right, Bottom int32
}

type POINT struct {
	X, Y int32
}

type WindowInfo struct {
	HWND        uintptr
	ProcessName string
	Title       string
	X, Y        int32
	Width       int32
	Height      int32
}

func GetForegroundWindow() uintptr {
	hwnd, _, _ := procGetForegroundWindow.Call()
	return hwnd
}

func GetWindowRect(hwnd uintptr) (RECT, error) {
	var rect RECT
	ret, _, err := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rect)))
	if ret == 0 {
		return rect, fmt.Errorf("GetWindowRect failed: %w", err)
	}
	return rect, nil
}

func SetWindowPos(hwnd uintptr, x, y int32) error {
	ret, _, err := procSetWindowPos.Call(
		hwnd,
		0,
		uintptr(x),
		uintptr(y),
		0, 0,
		SWP_NOSIZE|SWP_NOZORDER|SWP_NOACTIVATE|SWP_NOSENDCHANGING,
	)
	if ret == 0 {
		return fmt.Errorf("SetWindowPos failed: %w", err)
	}
	return nil
}

func GetWindowTitle(hwnd uintptr) string {
	buf := make([]uint16, 256)
	procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), 256)
	return syscall.UTF16ToString(buf)
}

func IsWindowVisible(hwnd uintptr) bool {
	ret, _, _ := procIsWindowVisible.Call(hwnd)
	return ret != 0
}

func EnumVisibleWindows() []WindowInfo {
	var wins []WindowInfo

	cb := syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		if !IsWindowVisible(hwnd) {
			return 1
		}
		pid := getWindowPID(hwnd)
		if pid == selfPID {
			return 1
		}
		title := GetWindowTitle(hwnd)
		if strings.TrimSpace(title) == "" {
			return 1
		}
		rect, err := GetWindowRect(hwnd)
		if err != nil {
			return 1
		}
		w := rect.Right - rect.Left
		h := rect.Bottom - rect.Top
		if w < 100 || h < 50 {
			return 1
		}
		wins = append(wins, WindowInfo{
			HWND:        hwnd,
			ProcessName: getProcessName(pid),
			Title:       title,
			X:           rect.Left,
			Y:           rect.Top,
			Width:       w,
			Height:      h,
		})
		return 1
	})

	procEnumWindows.Call(cb, 0)
	return wins
}

func GetCursorPos() (POINT, error) {
	var pt POINT
	ret, _, err := procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	if ret == 0 {
		return pt, fmt.Errorf("GetCursorPos failed: %w", err)
	}
	return pt, nil
}

func SetCursorPos(x, y int32) error {
	ret, _, err := procSetCursorPos.Call(uintptr(x), uintptr(y))
	if ret == 0 {
		return fmt.Errorf("SetCursorPos failed: %w", err)
	}
	return nil
}

// IsMouseButtonPressed 檢查滑鼠左鍵或右鍵是否正被按下
func IsMouseButtonPressed() bool {
	l, _, _ := procGetAsyncKeyState.Call(uintptr(0x01)) // VK_LBUTTON
	r, _, _ := procGetAsyncKeyState.Call(uintptr(0x02)) // VK_RBUTTON
	// GetAsyncKeyState 回傳值的最高位元(MSB)若為 1，表示按鍵當下處於按下狀態
	return (l&0x8000 != 0) || (r&0x8000 != 0)
}

// WindowFromPoint retrieves a handle to the window that contains the specified point.
// Uses Bitwise shift to safely pass POINT struct across x64 API call.
func WindowFromPoint(x, y int32) uintptr {
	ptVal := uintptr(uint32(x)) | (uintptr(uint32(y)) << 32)
	ret, _, _ := procWindowFromPoint.Call(ptVal)
	return ret
}

// GetAncestor retrieves the ancestor of the specified window (e.g. GA_ROOT for Top-Level Window).
func GetAncestor(hwnd uintptr, gaFlags uint32) uintptr {
	ret, _, _ := procGetAncestor.Call(hwnd, uintptr(gaFlags))
	return ret
}

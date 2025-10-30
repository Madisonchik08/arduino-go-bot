package screenfinder

import (
	"errors"
	"syscall"
	"unsafe"
)

type Color struct {
	R, G, B uint8
}

type Coord struct {
	X, Y int32
}

type Finder struct {
	PID         int
	HWND        uintptr
	Positions   []Coord
	TargetColor Color
}

var (
	user32      = syscall.NewLazyDLL("user32.dll")
	gdi32       = syscall.NewLazyDLL("gdi32.dll")
	enumWindows = user32.NewProc("EnumWindows")
	getWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	getDC      = user32.NewProc("GetDC")
	releaseDC  = user32.NewProc("ReleaseDC")
	getPixel   = gdi32.NewProc("GetPixel")
	getWindowRect = user32.NewProc("GetWindowRect")
)

type rect struct { Left, Top, Right, Bottom int32 }

// findWindowByPID ищет главное окно процесса по PID (не потокобезопасен).
func findWindowByPID(pid int) (hwnd uintptr, err error) {
	var foundHwnd uintptr
	cb := syscall.NewCallback(func(h syscall.Handle, lparam uintptr) uintptr {
		var procID uint32
		getWindowThreadProcessId.Call(uintptr(h), uintptr(unsafe.Pointer(&procID)))
		if int(procID) == pid {
			foundHwnd = uintptr(h)
			return 0 // terminate enumeration
		}
		return 1 // continue
	})
	enumWindows.Call(cb, 0)
	if foundHwnd != 0 {
		return foundHwnd, nil
	}
	return 0, errors.New("window not found by PID")
}

// Вызвать только один раз из main.go!
func (f *Finder) SetHWND() error {
	hwnd, err := findWindowByPID(f.PID)
	if err != nil {
		return err
	}
	f.HWND = hwnd
	return nil
}

func (f *Finder) TopLeft() (int32, int32, error) {
	if f.HWND == 0 { return 0,0, errors.New("HWND is not set") }
	var r rect
	rv, _, _ := getWindowRect.Call(f.HWND, uintptr(unsafe.Pointer(&r)))
	if rv == 0 { return 0,0, errors.New("GetWindowRect failed") }
	return r.Left, r.Top, nil
}

func (f *Finder) SetPositions(coords []Coord) { f.Positions = coords }

func (f *Finder) Find() (found bool, at Coord, err error) {
	if f.HWND == 0 {
		return false, Coord{}, errors.New("HWND is not set. Call SetHWND() first")
	}
	hdc, _, _ := getDC.Call(f.HWND)
	if hdc == 0 {
		return false, Coord{}, errors.New("getDC failed")
	}
	defer releaseDC.Call(f.HWND, hdc)

	for _, pos := range f.Positions {
		colorRef, _, _ := getPixel.Call(hdc, uintptr(pos.X), uintptr(pos.Y))
		r := uint8(colorRef & 0xFF)
		g := uint8((colorRef >> 8) & 0xFF)
		b := uint8((colorRef >> 16) & 0xFF)
		if r == f.TargetColor.R && g == f.TargetColor.G && b == f.TargetColor.B {
			return true, pos, nil
		}
	}
	return false, Coord{}, nil
}

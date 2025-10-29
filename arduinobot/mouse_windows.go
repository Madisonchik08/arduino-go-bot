package arduinobot

import (
	"golang.org/x/sys/windows"
	"unsafe"
)

// getMousePosition (неэкспортируемая) получает текущие координаты курсора.
func getMousePosition() (x, y int, err error) {
	var pt struct{ X, Y int32 }
	ret, _, callErr := windows.NewLazyDLL("user32.dll").NewProc("GetCursorPos").Call(uintptr(unsafe.Pointer(&pt)))
	if ret == 0 {
		return 0, 0, callErr
	}
	return int(pt.X), int(pt.Y), nil
}

package arduino_go_bot

import (
	"fmt"
	"log"
	"syscall"
)

const (
	f1                = 0xC2
	mouse_left_button = 1
)

func main() {
	arduinoDLL := syscall.NewLazyDLL("Arduino.dll")
	if arduinoDLL.Load() != nil {
		log.Fatal("Arduino DLL Load Error")
	}
	setPort := arduinoDLL.NewProc("set_port")
	keyPress := arduinoDLL.NewProc("key_press")
	mouseMove := arduinoDLL.NewProc("mouse_move")
	mouseClick := arduinoDLL.NewProc("mouse_click")

	fmt.Println("The Arduino.dll library and all functions were successfully loaded.")

	ret, _, err := setPort.Call(uintptr(3))
	if ret != 1 {
		log.Fatalf("Setting Arduino port failed: %s", err)
	}
	fmt.Println("Arduino port set successfully.")

	fmt.Println("Press F1...")
	ret, _, err = keyPress.Call(f1)
	if ret != 1 {
		log.Printf("Warning: keyPress function returned code %d, error: %v", ret, err)
	}
	fmt.Println("The F1 key is pressed.")

	fmt.Println("Move cursor in (100,100)...")
	ret, _, err = mouseMove.Call(uintptr(100), uintptr(100))
	if ret != 1 {
		log.Printf("Warning: mouseMove function returned code %d, error: %v", ret, err)
	}
	fmt.Println("The cursor has been moved.")

	fmt.Println("Click left button mouse...")
	ret, _, err = mouseClick.Call(uintptr(mouse_left_button))
	if ret != 1 {
		log.Printf("Warning: mouseClick function returned code %d, error: %v", ret, err)
	}
	fmt.Println("The mouse click has been clicked.")

	fmt.Println("\n All actions have been completed successfully!")
}

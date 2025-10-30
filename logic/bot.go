package logic

import (
	"log"
	"time"
	"arduino-go-bot/arduinobot"
	"arduino-go-bot/screenfinder"
)

const (
	KEY_F1     = 0xC2 // 194 F1
	KEY_F2     = 0xC3 // 195 F2
	MOUSE_LEFT = 1
)

var (
	errorCounter int
	maxErrors    = 5
)

func KeyPressRand(controller *arduinobot.Controller, key int) error {
	if err := controller.KeyDown(key); err != nil {
		return err
	}
	time.Sleep(300 * time.Millisecond)
	if err := controller.KeyUp(key); err != nil {
		return err
	}
	return nil
}

func ClickRand(controller *arduinobot.Controller, button int) error {
	if err := controller.MouseDown(button); err != nil {
		return err
	}
	time.Sleep(300 * time.Millisecond)
	if err := controller.MouseUp(button); err != nil {
		return err
	}
	return nil
}

func handleArduinoError(controllerPtr **arduinobot.Controller, prevConfig arduinobot.Config, err error) {
	errorCounter++
	log.Printf("Arduino error %d/%d: %v", errorCounter, maxErrors, err)
	if errorCounter < maxErrors {
		return
	}
	log.Println("Too many Arduino errors! Reconnecting controller...")
	(*controllerPtr).Close()
	time.Sleep(time.Second)
	newCtrl, err := arduinobot.NewController(prevConfig)
	if err != nil {
		log.Fatalf("Failed to reconnect to Arduino: %v", err)
	}
	*controllerPtr = newCtrl
	errorCounter = 0
	log.Println("Arduino connection successfully restarted")
}

func RunBotLoop(controller *arduinobot.Controller, prevConfig arduinobot.Config, finder *screenfinder.Finder) {
	log.Println("Bot started and searching...")
	for {
		found, coord, err := finder.Find()
		if err != nil {
			log.Printf("Window search error: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		if found {
			log.Printf("Monster found at %d,%d! Attacking...", coord.X, coord.Y)
			go func(c screenfinder.Coord) {
				if err := KeyPressRand(controller, KEY_F1); err != nil {
					handleArduinoError(&controller, prevConfig, err)
					return
				} else {
					errorCounter = 0
				}
				time.Sleep(300 * time.Millisecond)
				if err := controller.MouseMove(int(c.X), int(c.Y)); err != nil {
					handleArduinoError(&controller, prevConfig, err)
					return
				} else {
					errorCounter = 0
				}
				time.Sleep(300 * time.Millisecond)
				if err := ClickRand(controller, MOUSE_LEFT); err != nil {
					handleArduinoError(&controller, prevConfig, err)
					return
				} else {
					errorCounter = 0
				}
				time.Sleep(300 * time.Millisecond)
				killed := make(chan bool, 1)
				go func() {
					defer close(killed)
					for {
						time.Sleep(150 * time.Millisecond)
						check, _, _ := finder.Find()
						log.Printf("[DEBUG] Checking for monster disappearance in goroutine...")
						if !check {
							killed <- true
							break
						}
					}
				}()
				select {
				case <-time.After(6 * time.Second):
					log.Println("Monster not dead in 6s: teleporting (F2)")
					if err := KeyPressRand(controller, KEY_F2); err != nil {
						handleArduinoError(&controller, prevConfig, err)
						return
					} else {
						errorCounter = 0
					}
					time.Sleep(300 * time.Millisecond)
					log.Println("Waiting after fail-teleport...")
					time.Sleep(2500 * time.Millisecond)
					select {
					case <-killed:
						log.Printf("[DEBUG] Consumed leftover from killed to avoid blocking.")
					default:
					}
				case <-killed:
					log.Println("Monster killed in time, no teleport needed!")
					time.Sleep(300 * time.Millisecond)
				}
			}(coord)
		} else {
			log.Println("No monster found, auto-teleport (F2)")
			if err := KeyPressRand(controller, KEY_F2); err != nil {
				handleArduinoError(&controller, prevConfig, err)
				continue
			} else {
				errorCounter = 0
			}
			log.Println("Waiting after teleport...")
			time.Sleep(2500 * time.Millisecond)
		}
		time.Sleep(300 * time.Millisecond)
	}
}

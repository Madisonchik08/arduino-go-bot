package logic

import (
	"log"
	"math/rand"
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

func jitter(base, jitter time.Duration) time.Duration {
	if jitter <= 0 { return base }
	j := rand.Int63n(int64(jitter)+1)
	return base + time.Duration(j)
}

func KeyPressRand(controller *arduinobot.Controller, key int, actionDelay, actionJitter time.Duration) error {
	if err := controller.KeyDown(key); err != nil { return err }
	time.Sleep(jitter(actionDelay, actionJitter))
	if err := controller.KeyUp(key); err != nil { return err }
	return nil
}

func ClickRand(controller *arduinobot.Controller, button int, actionDelay, actionJitter time.Duration) error {
	if err := controller.MouseDown(button); err != nil { return err }
	time.Sleep(jitter(actionDelay, actionJitter))
	if err := controller.MouseUp(button); err != nil { return err }
	return nil
}

func handleArduinoError(controllerPtr **arduinobot.Controller, prevConfig arduinobot.Config, err error) {
	errorCounter++
	log.Printf("Temporary issue talking to Arduino (%d/%d). We will try to fix it automatically. Details: %v", errorCounter, maxErrors, err)
	if errorCounter < maxErrors { return }
	log.Println("Too many Arduino errors in a row. Reconnecting controller...")
	(*controllerPtr).Close()
	time.Sleep(time.Second)
	newCtrl, err := arduinobot.NewController(prevConfig)
	if err != nil { log.Fatalf("We couldn't reconnect to Arduino. Please check the USB cable and try again. Details: %v", err) }
	*controllerPtr = newCtrl
	errorCounter = 0
	log.Println("Arduino connection restored.")
}

// RunBotLoop runs until stopCh is closed. actionDelay/teleportDelay have optional jitters.
func RunBotLoop(controller *arduinobot.Controller, prevConfig arduinobot.Config, finder *screenfinder.Finder, stopCh <-chan struct{}, actionDelay, teleportDelay, actionJitter, teleportJitter time.Duration) {
	log.Println("App is running. Looking for monsters...")

	for {
		select { case <-stopCh: log.Println("Stopped by user."); return; default: }

		found, coord, err := finder.Find()
		if err != nil { log.Printf("We couldn't access the game window. We'll try again in a moment. Details: %v", err); time.Sleep(2*time.Second); continue }

		tlx, tly, _ := finder.TopLeft()

		if found {
			log.Printf("Monster found at %d,%d. Attacking...", coord.X, coord.Y)
			go func(c screenfinder.Coord) {
				if err := KeyPressRand(controller, KEY_F1, actionDelay, actionJitter); err != nil { handleArduinoError(&controller, prevConfig, err); return } else { errorCounter = 0 }
				time.Sleep(jitter(actionDelay, actionJitter))
				if err := controller.MouseMove(int(tlx+c.X), int(tly+c.Y)); err != nil { handleArduinoError(&controller, prevConfig, err); return } else { errorCounter = 0 }
				time.Sleep(jitter(actionDelay, actionJitter))
				if err := ClickRand(controller, MOUSE_LEFT, actionDelay, actionJitter); err != nil { handleArduinoError(&controller, prevConfig, err); return } else { errorCounter = 0 }
				time.Sleep(jitter(actionDelay, actionJitter))
				killed := make(chan bool, 1)
				go func(){ defer close(killed); for { select { case <-stopCh: return; default: } ; time.Sleep(150*time.Millisecond); check,_,_ := finder.Find(); if !check { killed<-true; break } } }()
				select {
				case <-stopCh:
					return
				case <-time.After(6 * time.Second):
					log.Println("Monster is still alive after 6s. Using teleport...")
					if err := KeyPressRand(controller, KEY_F2, actionDelay, actionJitter); err != nil { handleArduinoError(&controller, prevConfig, err); return } else { errorCounter = 0 }
					time.Sleep(jitter(actionDelay, actionJitter))
					log.Println("Waiting for the screen to update after teleport...")
					time.Sleep(jitter(teleportDelay, teleportJitter))
					select { case <-killed: default: }
				case <-killed:
					log.Println("Monster defeated. Ready for the next target!")
					time.Sleep(jitter(actionDelay, actionJitter))
				}
			}(coord)
		} else {
			log.Println("No monster detected. Performing auto-teleport...")
			if err := KeyPressRand(controller, KEY_F2, actionDelay, actionJitter); err != nil { handleArduinoError(&controller, prevConfig, err); continue } else { errorCounter = 0 }
			log.Println("Waiting for the screen to update after teleport...")
			time.Sleep(jitter(teleportDelay, teleportJitter))
		}
		time.Sleep(jitter(actionDelay, actionJitter))
	}
}

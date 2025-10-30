package main

import (
	"arduino-go-bot/arduinobot"
	"arduino-go-bot/logic"
	"arduino-go-bot/screenfinder"
	"log"
	"time"
)

func main() {
	prevConfig := arduinobot.Config{
		VID:         "2341",
		PID:         "8036",
		BaudRate:    115200,
		ReadTimeout: 2 * time.Second,
	}
	controller, err := arduinobot.NewController(prevConfig)
	if err != nil {
		log.Fatalf("Ошибка инициализации Arduino: %v", err)
	}
	defer controller.Close()

	finder := &screenfinder.Finder{
		PID:         12312,
		Positions:   []screenfinder.Coord{{X: 960, Y: 592}},
		TargetColor: screenfinder.Color{R: 255, G: 0, B: 0},
	}
	if err := finder.SetHWND(); err != nil {
		log.Fatalf("Failed to find game window: %v", err)
	}

	logic.RunBotLoop(controller, prevConfig, finder)
}

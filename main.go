package main

import (
	"arduino-go-bot/arduinobot"
	"arduino-go-bot/screenfinder"
	"log"
	"math/rand"
	"time"
)

// Коды клавиш (можно вынести в конфиг)
const (
	KEY_F1     = 0xC2 // scancode F1 (замени на свой код для Arduino)
	KEY_F2     = 0xC3 // scancode F2
	MOUSE_LEFT = 1    // кнопка мыши
)

func KeyPressRand(controller *arduinobot.Controller, key int) {
	controller.KeyDown(key)
	time.Sleep(time.Duration(100+rand.Intn(100)) * time.Millisecond)
	controller.KeyUp(key)
}

func ClickRand(controller *arduinobot.Controller, button int) {
	controller.MouseDown(button)
	time.Sleep(time.Duration(100+rand.Intn(100)) * time.Millisecond)
	controller.MouseUp(button)
}

func main() {
	// 1. Инициализация Arduino
	controller, err := arduinobot.NewController(arduinobot.Config{
		VID:         "2341",
		PID:         "8036",
		BaudRate:    115200,
		ReadTimeout: 2 * time.Second,
	})
	if err != nil {
		log.Fatalf("Ошибка инициализации Arduino: %v", err)
	}
	defer controller.Close()

	// 2. Finder для поиска цвета по координатам
	finder := &screenfinder.Finder{
		PID: 12312, // <-- вставляем твой PID
		Positions: []screenfinder.Coord{
			{960, 592},
		},
		TargetColor: screenfinder.Color{R: 255, G: 0, B: 0},
	}

	log.Println("Бот запущен и начал поиск...")
	for {
		found, coord, err := finder.Find()
		if err != nil {
			log.Printf("Ошибка поиска окна: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		if found {
			log.Printf("Монстр найден в %d,%d! Атакую...", coord.X, coord.Y)
			// Атака
			KeyPressRand(controller, KEY_F1)
			err = controller.MouseMove(int(coord.X), int(coord.Y))
			if err != nil {
				log.Printf("Ошибка MouseMove: %v", err)
			}
			ClickRand(controller, MOUSE_LEFT)

			// Проверяем, исчез ли цвет в течение 3 секунд
			killed := false
			for i := 0; i < 30; i++ {
				time.Sleep(100 * time.Millisecond)
				check, _, _ := finder.Find()
				if !check {
					killed = true
					break
				}
			}
			if !killed {
				log.Println("Монстр не убит — телепорт! (F2)")
				KeyPressRand(controller, KEY_F2)
				log.Println("Ожидание обновления экрана после телепорта...")
				time.Sleep(2500 * time.Millisecond)
			}

			// Пауза после атаки и клика
			log.Println("Пауза после атаки...")
			time.Sleep(600 * time.Millisecond)

		} else {
			// Монстр не найден
			log.Println("Монстр не найден на экране. Телепортируюсь (F2)...")
			KeyPressRand(controller, KEY_F2)
			log.Println("Ожидание обновления экрана после телепорта...")
			time.Sleep(2500 * time.Millisecond)
		}
	}
}

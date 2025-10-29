package main

import (
	"fmt"
	"io"
	"log"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
	"golang.org/x/sys/windows"
)

type Config struct {
	VID          string
	PID          string
	BaudRate     int
	ReadTimeout  time.Duration //Таймаут на ожидание ответа
	WriteTimeout time.Duration // Таймаут на запись
}
type Controller struct {
	config Config
	port   serial.Port
	mu     sync.Mutex
}

func NewController(config Config) (*Controller, error) {
	portName, err := findArduinoPort(config.VID, config.PID)
	if err != nil {
		return nil, err
	}
	log.Printf("Arduino найдена на порту: %s", portName)

	mode := &serial.Mode{
		BaudRate: config.BaudRate,
	}

	port, err := serial.Open(portName, mode)
	if err != nil {
		return nil, fmt.Errorf("не удалось открыть порт %s: %w", portName, err)
	}

	// Установка таймаутов
	if err := port.SetReadTimeout(config.ReadTimeout); err != nil {
		return nil, fmt.Errorf("не удалось установить таймаут на чтение: %w", err)
	}

	time.Sleep(2 * time.Second) // время на инициализацию после открытия порта
	return &Controller{
		config: config,
		port:   port,
	}, nil
}
func findArduinoPort(vid, pid string) (string, error) {
	ports, err := enumerator.GetDetailedPortsList()
	if err != nil {
		return "", err
	}
	for _, port := range ports {
		if port.IsUSB && strings.EqualFold(port.VID, vid) && strings.EqualFold(port.PID, pid) {
			return port.Name, nil
		}
	}
	return "", fmt.Errorf("устройство с VID=%s и PID=%s не найдено", vid, pid)
}
func (c *Controller) Close() {
	if c.port != nil {
		c.port.Close()
		log.Println("Соединение с портом закрыто.")
	}
}
func (c *Controller) sendAndReceive(cmd string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.port.ResetInputBuffer(); err != nil {
		return fmt.Errorf("не удалось очистить входной буфер: %w", err)
	}

	_, err := c.port.Write([]byte(cmd))
	if err != nil {
		return fmt.Errorf("ошибка при отправке команды '%s': %w", cmd, err)
	}
	log.Printf("Отправлена команда: %s", cmd)

	expectedReply := "ready"
	replyBuf := make([]byte, len(expectedReply))

	_, err = io.ReadFull(c.port, replyBuf)
	if err != nil {
		return fmt.Errorf("ошибка при чтении ответа на команду '%s': %w", cmd, err)
	}

	reply := string(replyBuf)
	log.Printf("Получен ответ: %s", reply)

	if reply != expectedReply {
		return fmt.Errorf("получен неожиданный ответ от Arduino: '%s'", reply)
	}

	return nil
}

//API

func (c *Controller) SetDelayKey(ms int) error {
	return c.sendAndReceive("00" + strconv.Itoa(ms))
}
func (c *Controller) SetDelayMouse(ms int) error {
	return c.sendAndReceive("01" + strconv.Itoa(ms))
}
func (c *Controller) SetDelayMouseMove(ms int) error {
	return c.sendAndReceive("02" + strconv.Itoa(ms))
}
func (c *Controller) SetOffsetMouseMove(step int) error {
	return c.sendAndReceive("03" + strconv.Itoa(step))
}
func (c *Controller) SetRandomDelayKey(rand int) error {
	return c.sendAndReceive("04" + strconv.Itoa(rand))
}
func (c *Controller) SetRandomDelayMouse(rand int) error {
	return c.sendAndReceive("05" + strconv.Itoa(rand))
}

// Команда '1': Нажать и отпустить клавишу
func (c *Controller) Key(code int) error {
	return c.sendAndReceive("1" + strconv.Itoa(code))
}

// Команда '2': Напечатать строку
func (c *Controller) Text(text string) error {
	return c.sendAndReceive("2" + text)
}

// Команда '3': Зажать клавишу
func (c *Controller) KeyDown(code int) error {
	return c.sendAndReceive("3" + strconv.Itoa(code))
}

// Команда '4': Отпустить клавишу
func (c *Controller) KeyUp(code int) error {
	return c.sendAndReceive("4" + strconv.Itoa(code))
}

// getMousePosition получает текущие координаты курсора (только для Windows).
func getMousePosition() (x, y int, err error) {
	var pt struct{ X, Y int32 }
	ret, _, callErr := windows.NewLazyDLL("user32.dll").NewProc("GetCursorPos").Call(uintptr(unsafe.Pointer(&pt)))
	if ret == 0 {
		return 0, 0, callErr
	}
	return int(pt.X), int(pt.Y), nil
}

// MouseMove перемещает курсор в абсолютные координаты.
func (c *Controller) MouseMove(targetX, targetY int) error {
	currentX, currentY, err := getMousePosition()
	if err != nil {
		return fmt.Errorf("не удалось получить текущую позицию мыши: %w", err)
	}

	deltaX := targetX - currentX
	deltaY := targetY - currentY

	znakX := "+"
	if deltaX < 0 {
		znakX = "-"
	}
	znakY := "+"
	if deltaY < 0 {
		znakY = "-"
	}
	value := int(math.Abs(float64(deltaX))*0xFFFF) + int(math.Abs(float64(deltaY)))
	cmd := fmt.Sprintf("5%s%s%d", znakX, znakY, value)

	return c.sendAndReceive(cmd)
}
func (c *Controller) MouseClick(button int) error {
	return c.sendAndReceive("6" + strconv.Itoa(button))
}
func (c *Controller) MouseDown(button int) error {
	return c.sendAndReceive("7" + strconv.Itoa(button))
}
func (c *Controller) MouseUp(button int) error {
	return c.sendAndReceive("8" + strconv.Itoa(button))
}
func (c *Controller) MouseWheel(amount int) error {
	// Положительные значения - вверх, отрицательные - вниз
	return c.sendAndReceive("9" + strconv.Itoa(amount))
}

// --- Основная программа для демонстрации ---

func main() {
	config := Config{
		// Нужно указать VID и PID платы
		VID:          "2341",
		PID:          "8036",
		BaudRate:     9600,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 2 * time.Second,
	}

	controller, err := NewController(config)
	if err != nil {
		log.Fatalf("Ошибка инициализации контроллера: %v", err)
	}
	defer controller.Close()

	log.Println("Контроллер готов. Выполняю демонстрационные действия...")

	// Пример 1: Нажать F1
	log.Println("Нажимаю клавишу F1...")
	if err := controller.Key(0xC2); err != nil { // 0xC2 = F1
		log.Printf("Ошибка: %v", err)
	}
	time.Sleep(1 * time.Second)

	// Пример 2: Переместить мышь в (300, 300) и кликнуть
	log.Println("Перемещаю мышь в (300, 300)...")
	if err := controller.MouseMove(300, 300); err != nil {
		log.Printf("Ошибка: %v", err)
	}
	time.Sleep(500 * time.Millisecond)

	log.Println("Кликаю левой кнопкой мыши...")
	if err := controller.MouseClick(1); err != nil { // 1 = левая кнопка
		log.Printf("Ошибка: %v", err)
	}
	time.Sleep(1 * time.Second)

	// Пример 3: Прокрутить колесико мыши вниз
	log.Println("Кручу колесико вниз...")
	if err := controller.MouseWheel(-3); err != nil { // отрицательное значение = вниз
		log.Printf("Ошибка: %v", err)
	}

	log.Println("\nДемонстрация завершена.")
}

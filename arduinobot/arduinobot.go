package arduinobot

import (
	"fmt"
	"io"
	"log"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
)

// Config содержит все настройки для подключения к Arduino.
type Config struct {
	VID         string
	PID         string
	BaudRate    int
	ReadTimeout time.Duration
}

// Controller управляет соединением и отправкой команд.
type Controller struct {
	config Config
	port   serial.Port
	mu     sync.Mutex
}

// NewController находит Arduino и создает готовый к работе контроллер.
func NewController(config Config) (*Controller, error) {
	portName, err := findArduinoPort(config.VID, config.PID)
	if err != nil {
		return nil, err
	}
	log.Printf("Arduino found on port: %s", portName)

	mode := &serial.Mode{
		BaudRate: config.BaudRate,
	}

	port, err := serial.Open(portName, mode)
	if err != nil {
		return nil, fmt.Errorf("failed to open port %s: %w", portName, err)
	}

	if err := port.SetReadTimeout(config.ReadTimeout); err != nil {
		return nil, fmt.Errorf("failed to set timeout for reading: %w", err)
	}

	time.Sleep(2 * time.Second)

	return &Controller{
		config: config,
		port:   port,
	}, nil
}

// findArduinoPort (неэкспортируемая) ищет COM-порт по VID и PID.
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
	return "", fmt.Errorf("device with VID=%s and PID=%s not found", vid, pid)
}

// Close корректно закрывает соединение с портом.
func (c *Controller) Close() {
	if c.port != nil {
		c.port.Close()
		log.Println("Connection to port closed.")
	}
}

// sendAndReceive (неэкспортируемая) отправляет команду и ожидает ответа "ready".
func (c *Controller) sendAndReceive(cmd string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.port.ResetInputBuffer(); err != nil {
		return fmt.Errorf("failed to clear input buffer: %w", err)
	}

	_, err := c.port.Write([]byte(cmd))
	if err != nil {
		return fmt.Errorf("error sending command '%s': %w", cmd, err)
	}
	log.Printf("Command sent: %s", cmd)

	expectedReply := "ready"
	replyBuf := make([]byte, len(expectedReply))
	done := make(chan error, 1)
	go func() {
		_, err := io.ReadFull(c.port, replyBuf)
		if err == nil && string(replyBuf) == expectedReply {
			done <- nil
		} else {
			done <- fmt.Errorf("no ready/error")
		}
	}()
	select {
	case err := <-done:
		if err != nil {
			log.Printf("[Arduino TIMEOUT ERROR] %s", err)
		}
		return err
	case <-time.After(700 * time.Millisecond):
		log.Printf("[Arduino TIMEOUT ERROR] timeout waiting for 'ready' on cmd '%s'", cmd)
		return fmt.Errorf("timeout waiting for Arduino ready")
	}
}

// --- Реализация API ---

func (c *Controller) SetDelayKey(ms int) error   { return c.sendAndReceive("00" + strconv.Itoa(ms)) }
func (c *Controller) SetDelayMouse(ms int) error { return c.sendAndReceive("01" + strconv.Itoa(ms)) }
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
func (c *Controller) Key(code int) error     { return c.sendAndReceive("1" + strconv.Itoa(code)) }
func (c *Controller) Text(text string) error { return c.sendAndReceive("2" + text) }
func (c *Controller) KeyDown(code int) error { return c.sendAndReceive("3" + strconv.Itoa(code)) }
func (c *Controller) KeyUp(code int) error   { return c.sendAndReceive("4" + strconv.Itoa(code)) }

func (c *Controller) MouseMove(targetX, targetY int) error {
	currentX, currentY, err := getMousePosition()
	if err != nil {
		return fmt.Errorf("failed to get current mouse position: %w", err)
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
	coordinate := int(math.Abs(float64(deltaX)))*65535 + int(math.Abs(float64(deltaY)))
	cmd := fmt.Sprintf("5%s%s%d", znakX, znakY, coordinate)
	return c.sendAndReceive(cmd)
}

func (c *Controller) MouseClick(button int) error {
	return c.sendAndReceive("6" + strconv.Itoa(button))
}
func (c *Controller) MouseDown(button int) error { return c.sendAndReceive("7" + strconv.Itoa(button)) }
func (c *Controller) MouseUp(button int) error   { return c.sendAndReceive("8" + strconv.Itoa(button)) }
func (c *Controller) MouseWheel(amount int) error {
	return c.sendAndReceive("9" + strconv.Itoa(amount))
}

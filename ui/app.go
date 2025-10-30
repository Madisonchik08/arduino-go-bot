package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"unsafe"
	"time"

	"arduino-go-bot/arduinobot"
	"arduino-go-bot/logic"
	"arduino-go-bot/screenfinder"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

type Config struct {
	ProcessName     string               `json:"processName"`
	Points          []screenfinder.Coord `json:"points"`
	ColorR          int                  `json:"colorR"`
	ColorG          int                  `json:"colorG"`
	ColorB          int                  `json:"colorB"`
	Hotkey          string               `json:"hotkey"`
	DelayMs         int                  `json:"delayMs"`
	DelayMsJitter   int                  `json:"delayMsJitter"`
	DelayF2Ms       int                  `json:"delayF2Ms"`
	DelayF2MsJitter int                  `json:"delayF2MsJitter"`
}

const configPath = "config.json"

// WinAPI bits for picking color/point and hotkey
var (
	user32                 = syscall.NewLazyDLL("user32.dll")
	gdi32                  = syscall.NewLazyDLL("gdi32.dll")
	procGetCursorPos       = user32.NewProc("GetCursorPos")
	procGetDC              = user32.NewProc("GetDC")
	procReleaseDC          = user32.NewProc("ReleaseDC")
	procGetPixel           = gdi32.NewProc("GetPixel")
	procRegisterHotKey     = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey   = user32.NewProc("UnregisterHotKey")
	procGetMessageW        = user32.NewProc("GetMessageW")
	procCreateToolhelpSnap = syscall.NewLazyDLL("kernel32.dll").NewProc("CreateToolhelp32Snapshot")
	procProcess32First     = syscall.NewLazyDLL("kernel32.dll").NewProc("Process32FirstW")
	procProcess32Next      = syscall.NewLazyDLL("kernel32.dll").NewProc("Process32NextW")
	procCloseHandle        = syscall.NewLazyDLL("kernel32.dll").NewProc("CloseHandle")
)

type point struct{ X, Y int32 }

type processEntry32 struct {
	dwSize              uint32
	cntUsage            uint32
	th32ProcessID       uint32
	th32DefaultHeapID   uintptr
	th32ModuleID        uint32
	cntThreads          uint32
	th32ParentProcessID uint32
	pcPriClassBase      int32
	dwFlags             uint32
	szExeFile           [260]uint16
}

const (
	TH32CS_SNAPPROCESS = 0x00000002
	MOD_ALT            = 0x0001
	MOD_CONTROL        = 0x0002
	MOD_SHIFT          = 0x0004
)

func getCursorPos() (point, error) {
	var p point
	r, _, e := procGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	if r == 0 {
		return p, e
	}
	return p, nil
}

func getScreenPixel(x, y int32) (r, g, b uint8, err error) {
	hdc, _, _ := procGetDC.Call(0)
	if hdc == 0 {
		return 0, 0, 0, fmt.Errorf("failed to get screen DC")
	}
	defer procReleaseDC.Call(0, hdc)
	clr, _, _ := procGetPixel.Call(hdc, uintptr(x), uintptr(y))
	return uint8(clr & 0xFF), uint8((clr >> 8) & 0xFF), uint8((clr >> 16) & 0xFF), nil
}

func findProcessIDByName(name string) int {
	hsnap, _, _ := procCreateToolhelpSnap.Call(TH32CS_SNAPPROCESS, 0)
	if hsnap == 0 {
		return 0
	}
	defer procCloseHandle.Call(hsnap)
	var e processEntry32
	e.dwSize = uint32(unsafe.Sizeof(e))
	r, _, _ := procProcess32First.Call(hsnap, uintptr(unsafe.Pointer(&e)))
	for r != 0 {
		exe := syscall.UTF16ToString(e.szExeFile[:])
		if strings.EqualFold(exe, name) {
			return int(e.th32ProcessID)
		}
		r, _, _ = procProcess32Next.Call(hsnap, uintptr(unsafe.Pointer(&e)))
	}
	return 0
}

func parseHotkey(s string) (mod, vk uint32) {
	s = strings.TrimSpace(strings.ToLower(s))
	if strings.Contains(s, "ctrl") {
		mod |= MOD_CONTROL
	}
	if strings.Contains(s, "shift") {
		mod |= MOD_SHIFT
	}
	if strings.Contains(s, "alt") {
		mod |= MOD_ALT
	}
	// default to "S"
	vk = 0x53
	parts := strings.Split(s, "+")
	if len(parts) > 0 {
		last := strings.TrimSpace(parts[len(parts)-1])
		if len(last) == 1 {
			vk = uint32(strings.ToUpper(last)[0])
		}
	}
	return
}

func loadConfig() *Config {
	cfg := &Config{
		ProcessName:     "Project Revenant.exe",
		Points:          []screenfinder.Coord{{X: 960, Y: 592}},
		ColorR:          255,
		ColorG:          0,
		ColorB:          0,
		Hotkey:          "Ctrl+Shift+S",
		DelayMs:         300,
		DelayMsJitter:   50,
		DelayF2Ms:       2500,
		DelayF2MsJitter: 200,
	}
	b, err := os.ReadFile(configPath)
	if err != nil { return cfg }
	_ = json.Unmarshal(b, cfg)
	return cfg
}

func saveConfig(cfg *Config) error {
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, b, 0644)
}

func listProcesses() []string {
	return []string{
		"Project Revenant.exe",
		"ragexe.exe",
		"game.exe",
	}
}

func parseInt(e *widget.Entry, def int) int {
	v, err := strconv.Atoi(strings.TrimSpace(e.Text))
	if err != nil {
		return def
	}
	return v
}

func listProcessesWithPID() map[string]int {
	res := map[string]int{}
	hsnap, _, _ := procCreateToolhelpSnap.Call(TH32CS_SNAPPROCESS, 0)
	if hsnap == 0 { return res }
	defer procCloseHandle.Call(hsnap)
	var e processEntry32
	e.dwSize = uint32(unsafe.Sizeof(e))
	r, _, _ := procProcess32First.Call(hsnap, uintptr(unsafe.Pointer(&e)))
	for r != 0 {
		name := syscall.UTF16ToString(e.szExeFile[:])
		res[name] = int(e.th32ProcessID)
		r, _, _ = procProcess32Next.Call(hsnap, uintptr(unsafe.Pointer(&e)))
	}
	return res
}

func Run() {
	cfg := loadConfig()
	ui := app.New()
	w := ui.NewWindow("Arduino GO")
	w.Resize(fyne.NewSize(640, 600))

	procMap := listProcessesWithPID()
	procNames := make([]string, 0, len(procMap))
	for n := range procMap { procNames = append(procNames, n) }
	processSelect := widget.NewSelect(procNames, func(string){})
	processSelect.PlaceHolder = "Select running process"
	processSelect.Selected = cfg.ProcessName
	refreshBtn := widget.NewButton("Refresh", func(){
		procMap = listProcessesWithPID()
		procNames = procNames[:0]
		for n := range procMap { procNames = append(procNames, n) }
		processSelect.Options = procNames
		processSelect.Refresh()
	})

	xEntry := widget.NewEntry(); xEntry.SetText(fmt.Sprintf("%d", cfg.Points[0].X))
	yEntry := widget.NewEntry(); yEntry.SetText(fmt.Sprintf("%d", cfg.Points[0].Y))

	rEntry := widget.NewEntry(); rEntry.SetText(fmt.Sprintf("%d", cfg.ColorR))
	gEntry := widget.NewEntry(); gEntry.SetText(fmt.Sprintf("%d", cfg.ColorG))
	bEntry := widget.NewEntry(); bEntry.SetText(fmt.Sprintf("%d", cfg.ColorB))
	delayEntry := widget.NewEntry(); delayEntry.SetText(fmt.Sprintf("%d", cfg.DelayMs))
	delayJitterEntry := widget.NewEntry(); delayJitterEntry.SetText(fmt.Sprintf("%d", cfg.DelayMsJitter))
	delayF2Entry := widget.NewEntry(); delayF2Entry.SetText(fmt.Sprintf("%d", cfg.DelayF2Ms))
	delayF2JitterEntry := widget.NewEntry(); delayF2JitterEntry.SetText(fmt.Sprintf("%d", cfg.DelayF2MsJitter))

	hotkeyEntry := widget.NewEntry(); hotkeyEntry.SetText(cfg.Hotkey)

	status := widget.NewLabel("Status: Stopped")

	var stopCh chan struct{}
	var running atomic.Bool
	var hotkeyID uint32 = 1
	var hotkeyRegistered bool

	startBtn := widget.NewButton("Start", nil)

	bindHotkeyBtn := widget.NewButton("Bind Hotkey", func() {
		mod, vk := parseHotkey(hotkeyEntry.Text)
		if hotkeyRegistered { procUnregisterHotKey.Call(0, uintptr(hotkeyID)); hotkeyRegistered=false }
		r, _, _ := procRegisterHotKey.Call(0, uintptr(hotkeyID), uintptr(mod), uintptr(vk))
		if r == 0 { status.SetText("Status: Failed to bind hotkey"); return }
		hotkeyRegistered = true
		status.SetText("Status: Hotkey bound")
		go func(){
			// Simple GetMessage loop to receive WM_HOTKEY (0x0312)
			type MSG struct { hwnd uintptr; message uint32; wParam uintptr; lParam uintptr; time uint32; pt struct{X,Y int32} }
			var m MSG
			for hotkeyRegistered {
				rv, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
				if rv == ^uintptr(0) { break }
				if m.message == 0x0312 {
					if running.Load() {
						if stopCh != nil { close(stopCh) }
						running.Store(false)
						status.SetText("Status: Stopped")
					} else {
						startBtn.OnTapped()
					}
				}
			}
		}()
	})

	pickPointBtn := widget.NewButton("Pick Point", func() {
		p, err := getCursorPos(); if err!=nil { status.SetText("Status: Failed to get cursor position"); return }
		xEntry.SetText(fmt.Sprintf("%d", p.X)); yEntry.SetText(fmt.Sprintf("%d", p.Y)); status.SetText("Status: Point captured")
	})

	pickColorBtn := widget.NewButton("Pick Color", func() {
		p, err := getCursorPos(); if err!=nil { status.SetText("Status: Failed to get cursor position"); return }
		r,g,b, err := getScreenPixel(p.X, p.Y); if err!=nil { status.SetText("Status: Failed to read pixel color"); return }
		rEntry.SetText(fmt.Sprintf("%d", r)); gEntry.SetText(fmt.Sprintf("%d", g)); bEntry.SetText(fmt.Sprintf("%d", b)); status.SetText("Status: Color captured")
	})

	startBtn.OnTapped = func() {
		if running.Load() { return }
		pn := processSelect.Selected
		pid := procMap[pn]
		if pid == 0 { status.SetText("Status: Select a running process"); return }
		x := int32(parseInt(xEntry, int(cfg.Points[0].X)))
		y := int32(parseInt(yEntry, int(cfg.Points[0].Y)))
		r := parseInt(rEntry, cfg.ColorR); g := parseInt(gEntry, cfg.ColorG); b := parseInt(bEntry, cfg.ColorB)
		delay := parseInt(delayEntry, cfg.DelayMs)
		delayJ := parseInt(delayJitterEntry, cfg.DelayMsJitter)
		delayF2 := parseInt(delayF2Entry, cfg.DelayF2Ms)
		delayF2J := parseInt(delayF2JitterEntry, cfg.DelayF2MsJitter)

		cfg.ProcessName = pn; cfg.Points = []screenfinder.Coord{{X:x,Y:y}}; cfg.ColorR, cfg.ColorG, cfg.ColorB = r,g,b; cfg.DelayMs = delay; cfg.DelayMsJitter = delayJ; cfg.DelayF2Ms = delayF2; cfg.DelayF2MsJitter = delayF2J; cfg.Hotkey = hotkeyEntry.Text; _=saveConfig(cfg)

		controller, err := arduinobot.NewController(arduinobot.Config{VID:"2341", PID:"8036", BaudRate:115200, ReadTimeout: 2*1e9})
		if err != nil { status.SetText(fmt.Sprintf("Status: Arduino error - %v", err)); return }

		finder := &screenfinder.Finder{PID: pid, Positions: []screenfinder.Coord{{X:x,Y:y}}, TargetColor: screenfinder.Color{R:uint8(r), G:uint8(g), B:uint8(b)}}
		if err := finder.SetHWND(); err != nil { status.SetText("Status: Game window not found."); controller.Close(); return }

		stopCh = make(chan struct{})
		go logic.RunBotLoop(
			controller,
			arduinobot.Config{VID:"2341", PID:"8036", BaudRate:115200},
			finder,
			stopCh,
			time.Duration(delay)*time.Millisecond,
			time.Duration(delayF2)*time.Millisecond,
			time.Duration(delayJ)*time.Millisecond,
			time.Duration(delayF2J)*time.Millisecond,
		)
		running.Store(true); status.SetText("Status: Running")
	}

	stopBtn := widget.NewButton("Stop", func(){ if !running.Load(){return}; close(stopCh); running.Store(false); status.SetText("Status: Stopped") })
	saveBtn := widget.NewButton("Save", func(){ cfg.ProcessName = processSelect.Selected; cfg.Points = []screenfinder.Coord{{X:int32(parseInt(xEntry,0)), Y:int32(parseInt(yEntry,0))}}; cfg.ColorR=parseInt(rEntry,0); cfg.ColorG=parseInt(gEntry,0); cfg.ColorB=parseInt(bEntry,0); cfg.DelayMs=parseInt(delayEntry,300); cfg.DelayMsJitter=parseInt(delayJitterEntry,50); cfg.DelayF2Ms=parseInt(delayF2Entry,2500); cfg.DelayF2MsJitter=parseInt(delayF2JitterEntry,200); cfg.Hotkey=hotkeyEntry.Text; _=saveConfig(cfg); status.SetText("Status: Settings saved") })

	form := container.NewVBox(
		widget.NewLabel("Process (running):"),
		container.NewHBox(processSelect, refreshBtn),
		widget.NewSeparator(),
		widget.NewLabel("Point (X,Y):"),
		container.NewHBox(xEntry, yEntry, pickPointBtn),
		widget.NewLabel("Color RGB:"),
		container.NewHBox(rEntry, gEntry, bEntry, pickColorBtn),
		widget.NewLabel("Global hotkey (e.g. Ctrl+Shift+S):"),
		container.NewHBox(hotkeyEntry, bindHotkeyBtn),
		widget.NewLabel("Action delay (ms) and jitter (ms):"),
		container.NewHBox(delayEntry, delayJitterEntry),
		widget.NewLabel("Delay after F2 (ms) and jitter (ms):"),
		container.NewHBox(delayF2Entry, delayF2JitterEntry),
		container.NewHBox(startBtn, stopBtn, saveBtn),
		status,
	)

	w.SetContent(form)
	w.ShowAndRun()
}

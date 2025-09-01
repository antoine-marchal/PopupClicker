// popup_clicker.go
package main

import (
	"bufio"
	"log"
	"os"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"github.com/lxn/win"
)

const (
	WINEVENT_OUTOFCONTEXT   = 0x0000
	WINEVENT_SKIPOWNPROCESS = 0x0002

	EVENT_SYSTEM_FOREGROUND = 0x0003
	EVENT_OBJECT_CREATE     = 0x8000

	BM_CLICK = 0x00F5
)

// defaults if .ini missing / keys absent
var (
	defaultTargetTitle = "Warning"
	defaultButtonText  = "OK"
	configPath         = "popup_clicker.ini"
)

var (
	targetTitle string
	buttonText  string

	user32            = syscall.NewLazyDLL("user32.dll")
	procGetWindowText = user32.NewProc("GetWindowTextW")
	procFindWindowEx  = user32.NewProc("FindWindowExW")
)

// simple INI parser: reads lines, supports optional [popup] section.
// It collects key=value pairs found in [popup] or in global scope.
// Keys are case-insensitive.
func loadINI(path string) map[string]string {
	cfg := make(map[string]string)
	f, err := os.Open(path)
	if err != nil {
		return cfg
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	section := ""
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		// comments ; or #
		if strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}
		// section header
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			continue
		}
		// key=value
		if idx := strings.IndexAny(line, "="); idx >= 0 {
			k := strings.TrimSpace(line[:idx])
			v := strings.TrimSpace(line[idx+1:])
			// strip surrounding quotes
			if len(v) >= 2 && ((v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'')) {
				v = v[1 : len(v)-1]
			}
			// we only accept keys from the [popup] section OR global
			if section == "" || section == "popup" {
				cfg[strings.ToLower(k)] = v
			}
		}
	}
	return cfg
}

// wrapper: GetWindowTextW -> returns string ("" if none)
func getWindowText(hwnd win.HWND) string {
	var buf [512]uint16
	ret, _, _ := procGetWindowText.Call(
		uintptr(hwnd),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
	)
	n := int(ret)
	if n == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf[:n])
}

// wrapper: FindWindowExW
func findWindowEx(parent, child win.HWND, className, windowName *uint16) win.HWND {
	ret, _, _ := procFindWindowEx.Call(
		uintptr(parent),
		uintptr(child),
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowName)),
	)
	return win.HWND(ret)
}

func winEventProc(hWinEventHook win.HWINEVENTHOOK, event uint32, hwnd win.HWND, idObject int32, idChild int32, dwEventThread uint32, dwmsEventTime uint32) {
	if hwnd == 0 {
		return
	}

	title := getWindowText(hwnd)
	if title == "" {
		return
	}

	// case-insensitive exact match; if you prefer substring matching,
	// swap to strings.Contains with lowercased strings.
	if !strings.EqualFold(title, targetTitle) {
		return
	}

	log.Printf("Matched window: hwnd=0x%x title=%q\n", hwnd, title)

	// iterate child Button controls and look for buttonText
	for child := findWindowEx(hwnd, 0, syscall.StringToUTF16Ptr("Button"), nil); child != 0; child = findWindowEx(hwnd, child, syscall.StringToUTF16Ptr("Button"), nil) {
		bt := getWindowText(child)
		if bt == "" {
			continue
		}
		// match ignoring case and optional accelerator '&'
		cleanBt := strings.TrimPrefix(bt, "&")
		if strings.EqualFold(cleanBt, buttonText) || strings.EqualFold(bt, buttonText) {
			// Send the click
			win.SendMessage(child, BM_CLICK, 0, 0)
			log.Printf("Sent BM_CLICK to child button hwnd=0x%x text=%q\n", child, bt)
			return
		}
	}

	// fallback
	win.PostMessage(hwnd, win.WM_COMMAND, 1, 0)
	log.Printf("Posted WM_COMMAND to hwnd=0x%x\n", hwnd)
}

func main() {
	// informational author line
	log.Println("popup_clicker — made by Antoine Marchal for CEA 2025/09")

	// load config
	cfg := loadINI(configPath)
	if v, ok := cfg["targettitle"]; ok && v != "" {
		targetTitle = v
	} else {
		targetTitle = defaultTargetTitle
	}
	if v, ok := cfg["buttontext"]; ok && v != "" {
		buttonText = v
	} else {
		buttonText = defaultButtonText
	}
	log.Printf("Config: targetTitle=%q buttonText=%q (ini=%s)\n", targetTitle, buttonText, configPath)

	// Keep callbacks on the same OS thread
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	var callback win.WINEVENTPROC = func(hHook win.HWINEVENTHOOK, event uint32, hwnd win.HWND, idObject int32, idChild int32, idEventThread uint32, dwmsEventTime uint32) uintptr {
		winEventProc(hHook, event, hwnd, idObject, idChild, idEventThread, dwmsEventTime)
		return 0
	}

	hook1, err := win.SetWinEventHook(EVENT_OBJECT_CREATE, EVENT_OBJECT_CREATE, 0, callback, 0, 0, WINEVENT_OUTOFCONTEXT|WINEVENT_SKIPOWNPROCESS)
	if err != nil || hook1 == 0 {
		log.Fatalf("SetWinEventHook(EVENT_OBJECT_CREATE) failed: %v", err)
	}
	hook2, err := win.SetWinEventHook(EVENT_SYSTEM_FOREGROUND, EVENT_SYSTEM_FOREGROUND, 0, callback, 0, 0, WINEVENT_OUTOFCONTEXT|WINEVENT_SKIPOWNPROCESS)
	if err != nil || hook2 == 0 {
		log.Fatalf("SetWinEventHook(EVENT_SYSTEM_FOREGROUND) failed: %v", err)
	}

	log.Println("Hook installed. Waiting for matching popups... (Ctrl+C to quit)")

	var msg win.MSG
	for win.GetMessage(&msg, 0, 0, 0) != 0 {
		win.TranslateMessage(&msg)
		win.DispatchMessage(&msg)
	}
}

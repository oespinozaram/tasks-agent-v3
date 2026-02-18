//go:build windows

package main

import (
	"path/filepath"
	"syscall"
	"unsafe"
)

var (
	user32                         = syscall.NewLazyDLL("user32.dll")
	kernel32                       = syscall.NewLazyDLL("kernel32.dll")
	procGetForegroundWindow        = user32.NewProc("GetForegroundWindow")
	procGetWindowTextW             = user32.NewProc("GetWindowTextW")
	procGetWindowThreadProcessId   = user32.NewProc("GetWindowThreadProcessId")
	procGetLastInputInfo           = user32.NewProc("GetLastInputInfo")
	procGetTickCount               = kernel32.NewProc("GetTickCount")
	procOpenProcess                = kernel32.NewProc("OpenProcess")
	procCloseHandle                = kernel32.NewProc("CloseHandle")
	procQueryFullProcessImageNameW = kernel32.NewProc("QueryFullProcessImageNameW")
)

// Estructura para detectar inactividad
type LASTINPUTINFO struct {
	cbSize uint32
	dwTime uint32
}

func GetSystemActivity() (ActivityInfo, error) {
	// 1. Obtener Inactividad
	var lastInput LASTINPUTINFO
	lastInput.cbSize = uint32(unsafe.Sizeof(lastInput))
	ret, _, _ := procGetLastInputInfo.Call(uintptr(unsafe.Pointer(&lastInput)))

	idleSeconds := 0.0
	if ret != 0 {
		currentTick, _, _ := procGetTickCount.Call()
		elapsed := currentTick - uintptr(lastInput.dwTime)
		idleSeconds = float64(elapsed) / 1000.0
	}

	// 2. Obtener Ventana Activa
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return ActivityInfo{AppName: "Unknown", WindowTitle: "Unknown", IdleSeconds: idleSeconds}, nil
	}

	// Título
	const bufSize = 256
	bufTitle := make([]uint16, bufSize)
	lenTitle, _, _ := procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&bufTitle[0])), uintptr(bufSize))
	title := syscall.UTF16ToString(bufTitle[:lenTitle])
	if title == "" {
		title = "Escritorio / Sin Título"
	}

	// Nombre del Proceso (.exe)
	var pid uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))

	const PROCESS_QUERY_LIMITED_INFORMATION = 0x1000
	hProcess, _, _ := procOpenProcess.Call(uintptr(PROCESS_QUERY_LIMITED_INFORMATION), 0, uintptr(pid))

	appName := "WindowsSystem"
	if hProcess != 0 {
		defer procCloseHandle.Call(hProcess)
		bufExe := make([]uint16, bufSize)
		lenExe := uint32(bufSize)
		ret2, _, _ := procQueryFullProcessImageNameW.Call(hProcess, 0, uintptr(unsafe.Pointer(&bufExe[0])), uintptr(unsafe.Pointer(&lenExe)))
		if ret2 != 0 {
			fullPath := syscall.UTF16ToString(bufExe[:lenExe])
			appName = filepath.Base(fullPath)
		}
	}

	return ActivityInfo{
		AppName:     appName,
		WindowTitle: title,
		IdleSeconds: idleSeconds,
	}, nil
}

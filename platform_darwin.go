//go:build darwin

package main

import (
	"os/exec"
	"strconv"
	"strings"
)

func GetSystemActivity() (ActivityInfo, error) {
	// 1. Obtener tiempo de inactividad (Idle Time) consultando al kernel
	idleSeconds := float64(0)
	if out, err := exec.Command("ioreg", "-c", "IOHIDSystem").Output(); err == nil {
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			if strings.Contains(line, "HIDIdleTime") {
				// El formato es: "HIDIdleTime" = 123456789 (en nanosegundos)
				parts := strings.Split(line, "=")
				if len(parts) == 2 {
					nanos, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
					if err == nil {
						idleSeconds = float64(nanos) / 1000000000.0
					}
				}
				break
			}
		}
	}

	// 2. Obtener App y Ventana usando AppleScript
	// Este script busca el proceso "frontmost" (el que está en primer plano)
	script := `
	tell application "System Events"
		set frontApp to first application process whose frontmost is true
		set appName to name of frontApp
		set winTitle to ""
		try
			set winTitle to name of front window of frontApp
		on error
			try
				set winTitle to value of attribute "AXTitle" of (first window of frontApp whose value of attribute "AXMain" is true)
			end try
		end try
		return appName & "|||" & winTitle
	end tell
	`

	cmd := exec.Command("osascript", "-e", script)
	out, err := cmd.Output()
	if err != nil {
		return ActivityInfo{
			AppName:     "Unknown",
			WindowTitle: "Unknown",
			IdleSeconds: idleSeconds,
		}, err
	}

	// 3. Limpiar y separar el resultado
	result := strings.TrimSpace(string(out))
	parts := strings.SplitN(result, "|||", 2)

	appName := "Unknown"
	windowTitle := "Unknown"

	if len(parts) > 0 && parts[0] != "" {
		appName = parts[0]
	}
	if len(parts) > 1 {
		windowTitle = parts[1]
		// macOS a veces devuelve un "missing value" en AppleScript si la ventana no tiene título nativo
		if windowTitle == "missing value" || windowTitle == "" {
			windowTitle = "Ventana Principal"
		}
	}

	return ActivityInfo{
		AppName:     appName,
		WindowTitle: windowTitle,
		IdleSeconds: idleSeconds,
	}, nil
}

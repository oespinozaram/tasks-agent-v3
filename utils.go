package main

import (
	"fmt"
	"os"
	"strings"
)

// IdentifyProcessSource determina si una app es Flatpak, Snap o Sistema
func IdentifyProcessSource(pid int) string {
	// 1. Intentar leer el enlace simbólico del ejecutable
	exePath, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
	if err != nil {
		return "Unknown (Kernel/System)"
	}

	// 2. Clasificar según la ruta
	if strings.Contains(exePath, "/flatpak/") || strings.Contains(exePath, ".flatpak") {
		return "Flatpak"
	}
	if strings.Contains(exePath, "/snap/") {
		return "Snap"
	}
	if strings.Contains(exePath, "/usr/bin") || strings.Contains(exePath, "/usr/lib") {
		return "System/Repo"
	}
	if strings.Contains(exePath, os.Getenv("HOME")) {
		return "User Local (AppImage/Manual)"
	}

	return "External/Other"
}

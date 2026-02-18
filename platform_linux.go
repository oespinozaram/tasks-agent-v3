//go:build linux

package main

func GetSystemActivity() (ActivityInfo, error) {
	// TODO: Implementar usando X11 o /proc
	// Recomendación futura: Usar librería "github.com/jezek/xgb" para X11
	return ActivityInfo{
		AppName:     "LinuxApp",
		WindowTitle: "Linux Window",
		IdleSeconds: 0,
	}, nil
}

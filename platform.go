package main

// ActivityInfo agrupa lo que necesitamos saber del sistema
type ActivityInfo struct {
	AppName     string
	WindowTitle string
	IdleSeconds float64 // Cuánto tiempo lleva sin mover el mouse/teclado
}

// Esta función debe implementarse en cada archivo platform_*.go
// Go vinculará la correcta al compilar.
// func GetSystemActivity() (ActivityInfo, error)

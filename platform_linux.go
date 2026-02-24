//go:build linux

package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/screensaver"
	"github.com/BurntSushi/xgb/xproto"
)

var (
	xConn            *xgb.Conn
	rootWindow       xproto.Window
	activeWindowAtom xproto.Atom
	wmNameAtom       xproto.Atom
	netWmNameAtom    xproto.Atom
	lastInitTime     time.Time
	screensaverInit  bool
	isWayland        bool
)

func init() {
	// Simple detection for Wayland environments
	if strings.Contains(strings.ToLower(os.Getenv("XDG_SESSION_TYPE")), "wayland") {
		isWayland = true
	}
}

// ensureX11Connection reconnects if the connection was lost or isn't init
func ensureX11Connection() error {
	if xConn != nil {
		return nil
	}

	// Throttle reconnection attempts to prevent spam
	if time.Since(lastInitTime) < 5*time.Second {
		return nil
	}
	lastInitTime = time.Now()

	// Redirect default logger for BurntSushi/xgb to discard noisy Authority warnings in Wayland
	if isWayland {
		xgb.Logger = log.New(io.Discard, "", 0)
	}

	client, err := xgb.NewConnDisplay("")
	if err != nil {
		return err
	}
	xConn = client

	setup := xproto.Setup(xConn)
	rootWindow = setup.DefaultScreen(xConn).Root

	// Initialize screensaver extension (Will fail silently or visibly without crash in Wayland)
	err = screensaver.Init(xConn)
	if err != nil {
		// Only log if we are NOT in Wayland OR if user wants debug
		if !isWayland {
			log.Println("Screensaver extension not available in X11:", err)
		}
		screensaverInit = false
	} else {
		screensaverInit = true
	}

	// Get necessary atoms
	activeWindowAtom = getAtom("_NET_ACTIVE_WINDOW")
	netWmNameAtom = getAtom("_NET_WM_NAME")
	wmNameAtom = getAtom("WM_NAME")

	return nil
}

func getAtom(name string) xproto.Atom {
	reply, err := xproto.InternAtom(xConn, true, uint16(len(name)), name).Reply()
	if err != nil {
		return 0
	}
	return reply.Atom
}

func GetSystemActivity() (ActivityInfo, error) {
	err := ensureX11Connection()
	if err != nil || xConn == nil {
		if isWayland {
			// Basic fallback for pure Wayland without xwayland where X11 completely fails
			return ActivityInfo{
				AppName:     "WaylandOS",
				WindowTitle: "No XWayland Window",
				IdleSeconds: 0,
			}, nil
		}

		return ActivityInfo{
			AppName:     "Unknown",
			WindowTitle: "Unknown",
			IdleSeconds: 0,
		}, err
	}

	// 1. Obtener Inactividad (Idle Time)
	idleSeconds := 0.0
	if screensaverInit {
		info, err := screensaver.QueryInfo(xConn, xproto.Drawable(rootWindow)).Reply()
		if err == nil {
			idleSeconds = float64(info.MsSinceUserInput) / 1000.0
		}
	}

	// 2. Obtener Ventana Activa
	activeWinReply, err := xproto.GetProperty(xConn, false, rootWindow, activeWindowAtom, xproto.GetPropertyTypeAny, 0, 1).Reply()
	if err != nil || len(activeWinReply.Value) == 0 {
		return ActivityInfo{
			AppName:     "Desktop",
			WindowTitle: "Desktop",
			IdleSeconds: idleSeconds,
		}, nil
	}

	activeWindow := xproto.Window(xgb.Get32(activeWinReply.Value))
	if activeWindow == 0 {
		return ActivityInfo{
			AppName:     "Desktop",
			WindowTitle: "Desktop",
			IdleSeconds: idleSeconds,
		}, nil
	}

	// 3. Obtener Título de la Ventana Activa (_NET_WM_NAME o WM_NAME)
	title := "Unknown"

	nameReply, err := xproto.GetProperty(xConn, false, activeWindow, netWmNameAtom, xproto.GetPropertyTypeAny, 0, 1024).Reply()
	if err == nil && len(nameReply.Value) > 0 {
		title = string(nameReply.Value)
	} else {
		nameReply, err = xproto.GetProperty(xConn, false, activeWindow, wmNameAtom, xproto.GetPropertyTypeAny, 0, 1024).Reply()
		if err == nil && len(nameReply.Value) > 0 {
			title = string(nameReply.Value)
		}
	}

	// 4. Obtener el nombre de la aplicación (Clase WM_CLASS)
	appName := "Unknown"
	wmClassAtom := getAtom("WM_CLASS")

	classReply, err := xproto.GetProperty(xConn, false, activeWindow, wmClassAtom, xproto.GetPropertyTypeAny, 0, 1024).Reply()
	if err == nil && len(classReply.Value) > 0 {
		parts := strings.Split(string(classReply.Value), "\x00")
		if len(parts) > 1 && parts[1] != "" {
			appName = parts[1]
		} else if len(parts) > 0 {
			appName = parts[0]
		}
	}

	// 2.5 Obtener PID y Origen
	pidAtom := getAtom("_NET_WM_PID")
	pidReply, err := xproto.GetProperty(xConn, false, activeWindow, pidAtom, xproto.GetPropertyTypeAny, 0, 1).Reply()

	var pid int
	if err == nil && len(pidReply.Value) > 0 {
		pid = int(xgb.Get32(pidReply.Value))
		source := IdentifyProcessSource(pid)
		// Puedes imprimirlo para debug:
		fmt.Printf("App: %s | Origen: %s | PID: %d\n", appName, source, pid)
	}

	return ActivityInfo{
		AppName:     appName,
		WindowTitle: title,
		IdleSeconds: idleSeconds,
	}, nil
}

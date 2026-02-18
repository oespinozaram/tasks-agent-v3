package main

import (
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"time"
)

// --- NUEVO: Función para cargar la lista blanca en memoria ---
func LoadWhitelist(db *sql.DB) map[string]bool {
	// Buscamos solo las apps habilitadas (is_enabled = 1)
	rows, err := db.Query("SELECT process_name FROM monitored_apps WHERE is_enabled = 1")
	if err != nil {
		// Si la tabla no existe aún o hay error, devolvemos mapa vacío (o podrías loguear el error)
		log.Println("⚠️ No se pudo cargar whitelist (¿Tabla vacía?):", err)
		return make(map[string]bool)
	}
	defer rows.Close()

	whitelist := make(map[string]bool)
	var name string
	for rows.Next() {
		if err := rows.Scan(&name); err == nil {
			whitelist[name] = true
		}
	}

	// Si la lista está vacía, podríamos decidir monitorear TODO por defecto
	// o no monitorear nada. Para este ejemplo, si está vacía, asumimos que
	// el usuario aún no configuró nada y retornamos vacío.
	return whitelist
}

func main() {
	// 1. Inicialización de Base de Datos
	db := InitDB()
	defer db.Close()

	// Corrección de fechas antiguas (si las hubiera)
	FixLegacyDates(db)

	fmt.Println("🕵️ Agente v3 (Local-First + Whitelist) Iniciado.")
	fmt.Println("📂 Guardando en:", dbPath)

	// 2. Cargar Configuración Inicial
	var idleThresholdStr string
	// Leemos umbral de inactividad
	err := db.QueryRow("SELECT value FROM agent_config WHERE key='idle_threshold_seconds'").Scan(&idleThresholdStr)
	if err != nil {
		idleThresholdStr = "300"
	} // Default si falla lectura

	idleThreshold, _ := strconv.ParseFloat(idleThresholdStr, 64)
	if idleThreshold == 0 {
		idleThreshold = 300
	}

	// --- NUEVO: Cargar la Whitelist inicial ---
	whitelist := LoadWhitelist(db)
	fmt.Printf("📋 Apps monitoreadas: %d\n", len(whitelist))

	// 3. Variables de Estado
	var lastInfo ActivityInfo
	var startTime = time.Now()
	var durationAccumulator = 0

	// 4. Temporizadores
	tickerSample := time.NewTicker(2 * time.Second)  // Muestreo rápido (Actividad)
	tickerConfig := time.NewTicker(60 * time.Second) // Muestreo lento (Recargar Config)

	defer tickerSample.Stop()
	defer tickerConfig.Stop()

	// 5. BUCLE PRINCIPAL (Usando select para múltiple escucha)
	for {
		select {
		// A) CASO RECARGA DE CONFIGURACIÓN (Cada 60s)
		case <-tickerConfig.C:
			// Recargamos la lista por si el usuario agregó algo en la UI
			newWhitelist := LoadWhitelist(db)
			// Solo avisamos si hubo cambios en la cantidad (opcional)
			if len(newWhitelist) != len(whitelist) {
				fmt.Printf("🔄 Configuración actualizada: %d apps en whitelist.\n", len(newWhitelist))
			}
			whitelist = newWhitelist

		// B) CASO MUESTREO DE ACTIVIDAD (Cada 2s)
		case <-tickerSample.C:
			currentInfo, err := GetSystemActivity()
			if err != nil {
				fmt.Println("Error capturando:", err)
				continue
			}

			// --- FILTRO 1: INACTIVIDAD (IDLE) ---
			if currentInfo.IdleSeconds > idleThreshold {
				fmt.Printf("\r💤 Inactivo (%.0fs) - Pausado...", currentInfo.IdleSeconds)

				// Si estábamos grabando algo antes de irnos idle, lo cerramos
				if durationAccumulator > 0 && lastInfo.AppName != "" {
					SaveLog(db, lastInfo, startTime, durationAccumulator)
					durationAccumulator = 0
					lastInfo = ActivityInfo{} // Reset total
				}
				continue
			}

			// --- FILTRO 2: WHITELIST (NUEVO) ---
			// Si la app actual NO está en la lista blanca
			if !whitelist[currentInfo.AppName] {
				// ¿Estábamos monitoreando algo válido antes? (Ej: Pasaste de VS Code a Spotify)
				if lastInfo.AppName != "" && durationAccumulator > 0 {
					// Guardamos lo que llevábamos de la app válida
					fmt.Printf("\n💾 Guardado (Fin de bloque válido): [%s] %ds", lastInfo.AppName, durationAccumulator)
					SaveLog(db, lastInfo, startTime, durationAccumulator)

					// Reseteamos contadores porque ahora estamos en "tierra de nadie"
					durationAccumulator = 0
					lastInfo = ActivityInfo{}
				}

				// Feedback visual opcional (para saber que el agente te está ignorando)
				// fmt.Printf("\r🙈 Ignorando: %s", currentInfo.AppName)
				continue
			}

			// --- LÓGICA DE CAMBIO DE CONTEXTO ---
			// Si llegamos aquí, la app ES válida y NO estamos idle.

			// ¿Cambió la App o el Título respecto a lo último registrado?
			if currentInfo.AppName != lastInfo.AppName || currentInfo.WindowTitle != lastInfo.WindowTitle {

				// Guardar el bloque anterior (si existía)
				if lastInfo.AppName != "" && durationAccumulator > 0 {
					fmt.Printf("\n💾 Guardado: [%s] %ds", lastInfo.AppName, durationAccumulator)
					SaveLog(db, lastInfo, startTime, durationAccumulator)
				}

				// Iniciar nuevo bloque con la nueva info
				lastInfo = currentInfo
				startTime = time.Now()
				durationAccumulator = 0

				fmt.Printf("\n👀 Grabando: %s | %s", currentInfo.AppName, currentInfo.WindowTitle)
			}

			// Acumulamos tiempo
			durationAccumulator += 2
		}
	}
}

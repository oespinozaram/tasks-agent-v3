package main

import (
	"database/sql"
	"log"
	"time"

	_ "modernc.org/sqlite"
)

var dbPath = "local_tracker.db"

func InitDB() *sql.DB {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatal(err)
	}

	// Habilitar WAL (Write-Ahead Logging) para concurrencia (Agente escribe, UI lee)
	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		log.Println("⚠️ No se pudo activar modo WAL:", err)
	}

	// 1. Tabla de Logs Crudos
	schemaLogs := `
	CREATE TABLE IF NOT EXISTS raw_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		app_name TEXT,
		window_title TEXT,
		start_time DATETIME,
		duration_seconds INTEGER,
		is_idle BOOLEAN DEFAULT 0
	);`

	// 2. Tabla de Configuración (Para que la UI la modifique y el agente lea)
	schemaConfig := `
	CREATE TABLE IF NOT EXISTS agent_config (
		key TEXT PRIMARY KEY,
		value TEXT
	);`

	schemaWhitelist := `
    CREATE TABLE IF NOT EXISTS monitored_apps (
        process_name TEXT PRIMARY KEY, 
        is_enabled BOOLEAN DEFAULT 1
    );`

	db.Exec(schemaLogs)
	db.Exec(schemaConfig)
	db.Exec(schemaWhitelist)

	// Configuración por defecto si no existe
	db.Exec("INSERT OR IGNORE INTO agent_config (key, value) VALUES ('idle_threshold_seconds', '300')")
	defaults := []string{"chrome.exe", "Code.exe", "devenv.exe", "pycharm64.exe", "WindowsTerminal.exe", "Antigravity.exe"}
	for _, app := range defaults {
		db.Exec("INSERT OR IGNORE INTO monitored_apps (process_name) VALUES (?)", app)
	}

	return db
}

func SaveLog(db *sql.DB, info ActivityInfo, startTime time.Time, duration int) {
	start := startTime.Format("2006-01-02 15:04:05")
	_, err := db.Exec(
		"INSERT INTO raw_logs (app_name, window_title, start_time, duration_seconds, is_idle) VALUES (?, ?, ?, ?, ?)",
		info.AppName, info.WindowTitle, start, duration, false,
	)
	if err != nil {
		log.Println("Error escribiendo DB:", err)
	}
}
func FixLegacyDates(db *sql.DB) {
	// Esta consulta SQL busca registros donde la fecha sea muy larga (formato sucio)
	// y la corta para dejar solo "YYYY-MM-DD HH:MM:SS"
	query := `
		UPDATE raw_logs 
		SET start_time = substr(start_time, 1, 19) 
		WHERE length(start_time) > 19;
	`

	res, err := db.Exec(query)
	if err != nil {
		log.Println("⚠️ Error intentando corregir fechas:", err)
		return
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected > 0 {
		log.Printf("✅ MIGRACIÓN: Se corrigieron %d registros con formato de fecha antiguo.\n", rowsAffected)
	}
}

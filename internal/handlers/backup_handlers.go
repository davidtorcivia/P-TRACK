package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"injection-tracker/internal/database"
	"injection-tracker/internal/middleware"
)

// BackupInfo represents information about a backup file
type BackupInfo struct {
	Filename  string `json:"filename"`
	Size      int64  `json:"size"`
	SizeHuman string `json:"size_human"`
	CreatedAt string `json:"created_at"`
	Path      string `json:"-"` // Internal use only
}

// AutoBackupSettings represents auto-backup configuration
type AutoBackupSettings struct {
	Enabled   bool   `json:"enabled"`
	Frequency string `json:"frequency"` // "daily" or "weekly"
	KeepCount int    `json:"keep_count"`
	LastRun   string `json:"last_run,omitempty"`
}

var (
	shutdownOnce sync.Once
	shutdownChan = make(chan struct{})
)

// getBackupDir returns the backup directory path, creating it if needed
func getBackupDir() (string, error) {
	backupDir := filepath.Join("data", "backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create backup directory: %w", err)
	}
	return backupDir, nil
}

// formatSize converts bytes to human-readable size
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// HandleListBackups returns list of available backup files
func HandleListBackups(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		if userID == 0 || !IsAdmin(db, userID) {
			http.Error(w, "Admin access required", http.StatusForbidden)
			return
		}

		backupDir, err := getBackupDir()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		entries, err := os.ReadDir(backupDir)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]BackupInfo{})
			return
		}

		backups := []BackupInfo{}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".db") {
				continue
			}

			info, err := entry.Info()
			if err != nil {
				continue
			}

			backups = append(backups, BackupInfo{
				Filename:  entry.Name(),
				Size:      info.Size(),
				SizeHuman: formatSize(info.Size()),
				CreatedAt: info.ModTime().Format("2006-01-02 15:04:05"),
				Path:      filepath.Join(backupDir, entry.Name()),
			})
		}

		sort.Slice(backups, func(i, j int) bool {
			return backups[i].CreatedAt > backups[j].CreatedAt
		})

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(backups)
	}
}

// HandleCreateBackup creates a new database backup
func HandleCreateBackup(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		if userID == 0 || !IsAdmin(db, userID) {
			http.Error(w, "Admin access required", http.StatusForbidden)
			return
		}

		backup, err := CreateBackup(db, "manual")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "Backup created successfully",
			"backup":  backup,
		})
	}
}

// CreateBackup creates a backup and returns info (used by both manual and auto-backup)
func CreateBackup(db *database.DB, prefix string) (*BackupInfo, error) {
	backupDir, err := getBackupDir()
	if err != nil {
		return nil, err
	}

	timestamp := time.Now().Format("2006-01-02_150405")
	backupFilename := fmt.Sprintf("%s_%s.db", prefix, timestamp)
	backupPath := filepath.Join(backupDir, backupFilename)

	// Use SQLite's backup mechanism via VACUUM INTO
	_, err = db.Exec(fmt.Sprintf("VACUUM INTO '%s'", backupPath))
	if err != nil {
		return nil, fmt.Errorf("failed to create backup: %w", err)
	}

	info, err := os.Stat(backupPath)
	if err != nil {
		return nil, fmt.Errorf("backup created but failed to get info: %w", err)
	}

	return &BackupInfo{
		Filename:  backupFilename,
		Size:      info.Size(),
		SizeHuman: formatSize(info.Size()),
		CreatedAt: info.ModTime().Format("2006-01-02 15:04:05"),
		Path:      backupPath,
	}, nil
}

// HandleDownloadBackup downloads a backup file
func HandleDownloadBackup(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		if userID == 0 || !IsAdmin(db, userID) {
			http.Error(w, "Admin access required", http.StatusForbidden)
			return
		}

		filename := r.URL.Query().Get("file")
		if filename == "" {
			http.Error(w, "Filename required", http.StatusBadRequest)
			return
		}

		filename = filepath.Base(filename)
		if !strings.HasSuffix(filename, ".db") {
			http.Error(w, "Invalid backup file", http.StatusBadRequest)
			return
		}

		backupDir, err := getBackupDir()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		backupPath := filepath.Join(backupDir, filename)

		absBackupDir, _ := filepath.Abs(backupDir)
		absPath, err := filepath.Abs(backupPath)
		if err != nil || !strings.HasPrefix(absPath, absBackupDir) {
			http.Error(w, "Invalid backup path", http.StatusBadRequest)
			return
		}

		file, err := os.Open(backupPath)
		if err != nil {
			http.Error(w, "Backup file not found", http.StatusNotFound)
			return
		}
		defer file.Close()

		info, _ := file.Stat()

		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))

		io.Copy(w, file)
	}
}

// HandleDeleteBackup deletes a backup file
func HandleDeleteBackup(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		if userID == 0 || !IsAdmin(db, userID) {
			http.Error(w, "Admin access required", http.StatusForbidden)
			return
		}

		var req struct {
			Filename string `json:"filename"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Filename == "" {
			http.Error(w, "Filename required", http.StatusBadRequest)
			return
		}

		filename := filepath.Base(req.Filename)
		if !strings.HasSuffix(filename, ".db") {
			http.Error(w, "Invalid backup file", http.StatusBadRequest)
			return
		}

		backupDir, err := getBackupDir()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		backupPath := filepath.Join(backupDir, filename)

		absBackupDir, _ := filepath.Abs(backupDir)
		absPath, err := filepath.Abs(backupPath)
		if err != nil || !strings.HasPrefix(absPath, absBackupDir) {
			http.Error(w, "Invalid backup path", http.StatusBadRequest)
			return
		}

		if err := os.Remove(backupPath); err != nil {
			http.Error(w, "Failed to delete backup", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "Backup deleted successfully",
			"success": true,
		})
	}
}

// HandleUploadBackup handles backup file upload for restore
func HandleUploadBackup(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		if userID == 0 || !IsAdmin(db, userID) {
			http.Error(w, "Admin access required", http.StatusForbidden)
			return
		}

		// Limit upload size to 100MB
		r.Body = http.MaxBytesReader(w, r.Body, 100<<20)

		file, header, err := r.FormFile("backup")
		if err != nil {
			http.Error(w, "Failed to read uploaded file", http.StatusBadRequest)
			return
		}
		defer file.Close()

		if !strings.HasSuffix(header.Filename, ".db") {
			http.Error(w, "Invalid file type. Must be a .db file", http.StatusBadRequest)
			return
		}

		backupDir, err := getBackupDir()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Save uploaded file to staging area
		stagingPath := filepath.Join(backupDir, "restore_staging.db")
		out, err := os.Create(stagingPath)
		if err != nil {
			http.Error(w, "Failed to save uploaded file", http.StatusInternalServerError)
			return
		}

		_, err = io.Copy(out, file)
		out.Close()
		if err != nil {
			os.Remove(stagingPath)
			http.Error(w, "Failed to save uploaded file", http.StatusInternalServerError)
			return
		}

		// Validate it's a valid SQLite database
		testDB, err := sql.Open("sqlite3", stagingPath+"?mode=ro")
		if err != nil {
			os.Remove(stagingPath)
			http.Error(w, "Invalid database file", http.StatusBadRequest)
			return
		}

		// Try a simple query to validate
		var count int
		err = testDB.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table'").Scan(&count)
		testDB.Close()
		if err != nil || count == 0 {
			os.Remove(stagingPath)
			http.Error(w, "Invalid or empty database file", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":      "Backup uploaded and validated. Ready to restore.",
			"staging_file": "restore_staging.db",
			"success":      true,
		})
	}
}

// HandleRestoreBackup performs the actual restore and triggers server restart
func HandleRestoreBackup(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		if userID == 0 || !IsAdmin(db, userID) {
			http.Error(w, "Admin access required", http.StatusForbidden)
			return
		}

		var req struct {
			Filename string `json:"filename"`
			Confirm  bool   `json:"confirm"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		if !req.Confirm {
			http.Error(w, "Confirmation required", http.StatusBadRequest)
			return
		}

		backupDir, err := getBackupDir()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Determine source file
		var sourcePath string
		if req.Filename == "" || req.Filename == "restore_staging.db" {
			sourcePath = filepath.Join(backupDir, "restore_staging.db")
		} else {
			sourcePath = filepath.Join(backupDir, filepath.Base(req.Filename))
		}

		if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
			http.Error(w, "Backup file not found", http.StatusNotFound)
			return
		}

		// Create pre-restore backup
		_, err = CreateBackup(db, "pre_restore")
		if err != nil {
			http.Error(w, "Failed to create pre-restore backup: "+err.Error(), http.StatusInternalServerError)
			return
		}

		dbPath := filepath.Join("data", "tracker.db")

		// Close database and perform restore
		// We'll copy the file and then signal for restart
		restorePath := filepath.Join(backupDir, "pending_restore.db")

		// Copy source to pending restore location
		src, err := os.Open(sourcePath)
		if err != nil {
			http.Error(w, "Failed to open backup file", http.StatusInternalServerError)
			return
		}
		defer src.Close()

		dst, err := os.Create(restorePath)
		if err != nil {
			http.Error(w, "Failed to prepare restore", http.StatusInternalServerError)
			return
		}

		_, err = io.Copy(dst, src)
		dst.Close()
		if err != nil {
			os.Remove(restorePath)
			http.Error(w, "Failed to prepare restore", http.StatusInternalServerError)
			return
		}

		// Write a restore flag file that main.go can check on startup
		flagPath := filepath.Join("data", "pending_restore")
		os.WriteFile(flagPath, []byte(restorePath), 0644)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "Restore prepared. Server will restart now. Please wait and refresh the page.",
			"success": true,
		})

		// Trigger graceful shutdown after response is sent
		go func() {
			time.Sleep(500 * time.Millisecond)

			// Perform the actual file swap
			db.Close()
			time.Sleep(100 * time.Millisecond)

			// Backup current DB
			os.Rename(dbPath, dbPath+".pre_restore")

			// Move pending restore to main DB
			os.Rename(restorePath, dbPath)

			// Remove flag file
			os.Remove(flagPath)

			// Exit - process manager should restart us
			os.Exit(0)
		}()
	}
}

// HandleGetAutoBackupSettings returns auto-backup configuration
func HandleGetAutoBackupSettings(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		if userID == 0 || !IsAdmin(db, userID) {
			http.Error(w, "Admin access required", http.StatusForbidden)
			return
		}

		settings := getAutoBackupSettings(db)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(settings)
	}
}

// HandleUpdateAutoBackupSettings updates auto-backup configuration
func HandleUpdateAutoBackupSettings(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		if userID == 0 || !IsAdmin(db, userID) {
			http.Error(w, "Admin access required", http.StatusForbidden)
			return
		}

		var req AutoBackupSettings
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		// Validate
		if req.Frequency != "" && req.Frequency != "daily" && req.Frequency != "weekly" {
			http.Error(w, "Frequency must be 'daily' or 'weekly'", http.StatusBadRequest)
			return
		}
		if req.KeepCount < 1 {
			req.KeepCount = 7
		}
		if req.KeepCount > 100 {
			req.KeepCount = 100
		}

		now := time.Now()
		db.Exec(`INSERT INTO settings (key, value, updated_at, updated_by) VALUES (?, ?, ?, ?)
			ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
			"auto_backup_enabled", fmt.Sprintf("%t", req.Enabled), now, userID)
		db.Exec(`INSERT INTO settings (key, value, updated_at, updated_by) VALUES (?, ?, ?, ?)
			ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
			"auto_backup_frequency", req.Frequency, now, userID)
		db.Exec(`INSERT INTO settings (key, value, updated_at, updated_by) VALUES (?, ?, ?, ?)
			ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
			"auto_backup_keep_count", fmt.Sprintf("%d", req.KeepCount), now, userID)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":  "Auto-backup settings saved",
			"settings": getAutoBackupSettings(db),
		})
	}
}

func getAutoBackupSettings(db *database.DB) *AutoBackupSettings {
	settings := &AutoBackupSettings{
		Frequency: "daily",
		KeepCount: 7,
	}

	var value string
	if err := db.QueryRow("SELECT value FROM settings WHERE key = 'auto_backup_enabled'").Scan(&value); err == nil {
		settings.Enabled = value == "true"
	}
	if err := db.QueryRow("SELECT value FROM settings WHERE key = 'auto_backup_frequency'").Scan(&value); err == nil && value != "" {
		settings.Frequency = value
	}
	if err := db.QueryRow("SELECT value FROM settings WHERE key = 'auto_backup_keep_count'").Scan(&value); err == nil {
		fmt.Sscanf(value, "%d", &settings.KeepCount)
	}
	if err := db.QueryRow("SELECT value FROM settings WHERE key = 'auto_backup_last_run'").Scan(&value); err == nil {
		settings.LastRun = value
	}

	return settings
}

// PruneOldBackups removes old auto-backups beyond the keep count
func PruneOldBackups(db *database.DB) error {
	settings := getAutoBackupSettings(db)
	if settings.KeepCount <= 0 {
		return nil
	}

	backupDir, err := getBackupDir()
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return err
	}

	// Collect auto-backups only
	var autoBackups []os.DirEntry
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), "auto_") && strings.HasSuffix(entry.Name(), ".db") {
			autoBackups = append(autoBackups, entry)
		}
	}

	// Sort by modification time (newest first)
	sort.Slice(autoBackups, func(i, j int) bool {
		infoI, _ := autoBackups[i].Info()
		infoJ, _ := autoBackups[j].Info()
		return infoI.ModTime().After(infoJ.ModTime())
	})

	// Delete old backups beyond keep count
	for i := settings.KeepCount; i < len(autoBackups); i++ {
		os.Remove(filepath.Join(backupDir, autoBackups[i].Name()))
	}

	return nil
}

// RunAutoBackup checks if an auto-backup is needed and runs it
func RunAutoBackup(db *database.DB) error {
	settings := getAutoBackupSettings(db)
	if !settings.Enabled {
		return nil
	}

	// Check if backup is needed
	needsBackup := false
	if settings.LastRun == "" {
		needsBackup = true
	} else {
		lastRun, err := time.Parse("2006-01-02 15:04:05", settings.LastRun)
		if err != nil {
			needsBackup = true
		} else {
			var threshold time.Duration
			if settings.Frequency == "weekly" {
				threshold = 7 * 24 * time.Hour
			} else {
				threshold = 24 * time.Hour
			}
			needsBackup = time.Since(lastRun) >= threshold
		}
	}

	if !needsBackup {
		return nil
	}

	// Create backup
	_, err := CreateBackup(db, "auto")
	if err != nil {
		return err
	}

	// Update last run time
	now := time.Now().Format("2006-01-02 15:04:05")
	db.Exec(`INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		"auto_backup_last_run", now, now)

	// Prune old backups
	PruneOldBackups(db)

	return nil
}

// StartAutoBackupScheduler starts the background auto-backup scheduler
func StartAutoBackupScheduler(db *database.DB) {
	// Run immediately on startup
	go func() {
		time.Sleep(10 * time.Second) // Wait for server to fully start
		RunAutoBackup(db)
	}()

	// Then run every hour to check
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				RunAutoBackup(db)
			case <-shutdownChan:
				return
			}
		}
	}()
}

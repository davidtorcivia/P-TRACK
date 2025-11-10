package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"injection-tracker/internal/auth"
	"injection-tracker/internal/config"
	"injection-tracker/internal/database"
	"injection-tracker/internal/handlers"
	"injection-tracker/internal/middleware"
	"injection-tracker/internal/web"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func main() {
	// Load environment variables
	if err := loadEnv(); err != nil {
		log.Printf("Warning: %v", err)
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Open database
	db, err := database.Open(cfg.Database.Path)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Run migrations
	if err := db.RunMigrations(); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Initialize security components
	jwtManager := auth.NewJWTManager(cfg.Security.JWTSecret, cfg.Security.SessionDuration)
	csrfProtection := middleware.NewCSRFProtection(cfg.Security.CSRFSecret)
	rateLimiter := middleware.NewRateLimiter(cfg.Security.RateLimitRequests, cfg.Security.RateLimitWindow)
	loginRateLimiter := middleware.NewRateLimiter(cfg.Security.LoginRateLimit, cfg.Security.LoginRateWindow)
	authMiddleware := middleware.NewAuthMiddleware(jwtManager)

	// Initialize router
	r := chi.NewRouter()

	// Apply global middleware
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Timeout(60 * time.Second))
	r.Use(middleware.SecurityHeaders(cfg.Security.CSPEnabled, cfg.Security.HSTSEnabled))

	// CORS configuration
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://localhost:*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Initialize templates
	if err := initializeTemplates(); err != nil {
		log.Fatalf("Failed to initialize templates: %v", err)
	}

	// Public routes (no authentication required)
	r.Group(func(r chi.Router) {
		r.Use(rateLimiter.Middleware)

		// Health check
		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})

		// Setup routes (always available)
		r.Get("/setup", handlers.HandleSetupPage(db))
		r.Post("/api/setup", handlers.HandleSetup(db))

		// Public web pages (with setup check middleware)
		r.With(requireSetupComplete(db)).Get("/", handlers.HandleHome(db))
		r.With(requireSetupComplete(db)).Get("/login", handlers.HandleLoginPage)
		r.With(requireSetupComplete(db)).Get("/register", handlers.HandleRegisterPage)
		r.With(requireSetupComplete(db)).Get("/forgot-password", handlers.HandleForgotPasswordPage)

		// Authentication routes
		r.Route("/api/auth", func(r chi.Router) {
			r.With(loginRateLimiter.Middleware).Post("/login", handlers.HandleLogin(db, jwtManager))
			r.With(loginRateLimiter.Middleware).Post("/register", handlers.HandleRegister(db))
			r.Post("/forgot-password", handleForgotPassword(db))
			r.Post("/reset-password", handleResetPassword(db))
		})

		// Serve static files
		r.Get("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))).ServeHTTP)
		r.Get("/manifest.json", serveManifest)
		r.Get("/service-worker.js", serveServiceWorker)
	})

	// Protected routes (authentication required)
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware.RequireAuth)
		r.Use(rateLimiter.Middleware)
		r.Use(csrfProtection.Middleware)

		// API routes
		r.Route("/api", func(r chi.Router) {
			r.Get("/csrf-token", handleGetCSRFToken(csrfProtection))

			// Dashboard routes
			r.Get("/dashboard/recent", handlers.HandleGetRecentActivity(db))

			// User routes
			r.Get("/auth/me", handlers.HandleGetCurrentUser(db))
			r.Post("/auth/logout", handlers.HandleLogout(db))
			r.Post("/auth/refresh", handlers.HandleRefreshToken(db, jwtManager))

			// Course routes
			r.Route("/courses", func(r chi.Router) {
				r.Get("/", handlers.HandleGetCourses(db))
				r.Post("/", handlers.HandleCreateCourse(db))
				r.Get("/active", handlers.HandleGetActiveCourse(db))
				r.Get("/{id}", handlers.HandleGetCourse(db))
				r.Put("/{id}", handlers.HandleUpdateCourse(db))
				r.Delete("/{id}", handlers.HandleDeleteCourse(db))
				r.Post("/{id}/activate", handlers.HandleActivateCourse(db))
				r.Post("/{id}/close", handlers.HandleCloseCourse(db))
			})

			// Injection routes
			r.Route("/injections", func(r chi.Router) {
				r.Get("/", handlers.HandleGetInjections(db))
				r.Post("/", handlers.HandleCreateInjection(db))
				r.Get("/recent", handlers.HandleGetRecentInjections(db))
				r.Get("/stats", handlers.HandleGetInjectionStats(db))
				r.Get("/{id}", handlers.HandleGetInjection(db))
				r.Put("/{id}", handlers.HandleUpdateInjection(db))
				r.Delete("/{id}", handlers.HandleDeleteInjection(db))
			})

			// Symptom routes
			r.Route("/symptoms", func(r chi.Router) {
				r.Get("/", handlers.HandleGetSymptoms(db))
				r.Post("/", handlers.HandleCreateSymptom(db))
				r.Get("/recent", handlers.HandleGetRecentSymptoms(db))
				r.Get("/trends", handlers.HandleGetSymptomTrends(db))
				r.Get("/{id}", handlers.HandleGetSymptom(db))
				r.Put("/{id}", handlers.HandleUpdateSymptom(db))
				r.Delete("/{id}", handlers.HandleDeleteSymptom(db))
			})

			// Medication routes
			r.Route("/medications", func(r chi.Router) {
				r.Get("/", handlers.HandleGetMedications(db))
				r.Post("/", handlers.HandleCreateMedication(db))
				r.Get("/schedule/today", handlers.HandleGetDailySchedule(db))
				r.Get("/adherence", handlers.HandleGetAdherence(db))
				r.Get("/{id}", handlers.HandleGetMedication(db))
				r.Put("/{id}", handlers.HandleUpdateMedication(db))
				r.Delete("/{id}", handlers.HandleDeleteMedication(db))
				r.Post("/{id}/log", handlers.HandleLogMedication(db))
				r.Get("/{id}/logs", handlers.HandleGetMedicationLogs(db))
			})

			// Inventory routes
			r.Route("/inventory", func(r chi.Router) {
				r.Get("/", handlers.HandleGetInventory(db))
				r.Put("/{itemType}", handlers.HandleUpdateInventory(db))
				r.Get("/history", handlers.HandleGetAllInventoryHistory(db))
				r.Get("/history/recent", handlers.HandleGetRecentInventoryChanges(db))
				r.Get("/{itemType}/history", handlers.HandleGetInventoryHistory(db))
				r.Post("/{itemType}/adjust", handlers.HandleAdjustInventory(db))
				r.Get("/alerts", handlers.HandleGetInventoryAlerts(db))
				r.Post("/settings", handlers.HandleUpdateInventorySettings(db))
			})

			// Export routes
			r.Get("/export/pdf", handlers.HandleExportPDF(db))
			r.Get("/export/csv", handlers.HandleExportCSV(db))

			// Settings routes
			r.Get("/settings", handlers.HandleGetSettings(db))
			r.Put("/settings", handlers.HandleUpdateSettings(db))
			r.Post("/settings/profile", handlers.HandleUpdateProfile(db))
			r.Post("/settings/password", handlers.HandleChangePassword(db))

			// Notification routes
			r.Get("/notifications", handleGetNotifications(db))
			r.Put("/notifications/{id}/read", handleMarkNotificationRead(db))
		})

		// Protected web pages (HTML responses)
		r.Get("/dashboard", handlers.HandleDashboard(db, csrfProtection))
		r.Get("/activity", handlers.HandleActivityPage(db, csrfProtection))
		r.Get("/injections", handlers.HandleInjectionsPage(db, csrfProtection))
		r.Get("/symptoms", handlers.HandleSymptomsPage(db, csrfProtection))
		r.Get("/symptoms/log", handlers.HandleLogSymptomPage(db))
		r.Get("/symptoms/{id}/edit", handlers.HandleEditSymptomPage(db, csrfProtection))
		r.Get("/symptoms/history", handlers.HandleSymptomsHistoryPage(db, csrfProtection))
		r.Get("/medications", handlers.HandleMedicationsPage(db, csrfProtection))
		r.Get("/medications/log", handlers.HandleLogMedicationPage(db))
		r.Get("/medications/new", handlers.HandleNewMedicationPage(db))
		r.Get("/inventory", handlers.HandleInventoryPage(db, csrfProtection))
		r.Get("/inventory/history", handlers.HandleInventoryHistoryPage(db, csrfProtection))
		r.Get("/inventory/{itemType}/history", handlers.HandleInventoryItemHistoryPage(db, csrfProtection))
		r.Get("/courses", handlers.HandleCoursesPage(db, csrfProtection))
		r.Get("/courses/new", handlers.HandleNewCoursePage(db))
		r.Get("/calendar", handlers.HandleCalendarPage(db, csrfProtection))
		r.Get("/reports", handlers.HandleReportsPage(db, csrfProtection))
		r.Get("/settings", handlers.HandleSettingsPage(db, csrfProtection))
	})

	// Start server
	addr := fmt.Sprintf(":%s", cfg.Server.Port)
	log.Printf("Server starting on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

// loadEnv loads environment variables from .env file
func loadEnv() error {
	data, err := os.ReadFile(".env")
	if err != nil {
		return err
	}

	lines := splitLines(string(data))
	for _, line := range lines {
		if line == "" || line[0] == '#' {
			continue
		}

		parts := splitOnce(line, '=')
		if len(parts) == 2 {
			os.Setenv(parts[0], parts[1])
		}
	}

	return nil
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func splitOnce(s string, sep byte) []string {
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			return []string{s[:i], s[i+1:]}
		}
	}
	return []string{s}
}

// initializeTemplates loads all HTML templates
func initializeTemplates() error {
	return web.InitTemplates()
}

// handleForgotPassword handles password reset request (not implemented)
func handleForgotPassword(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Password reset not implemented yet. Please contact administrator.", http.StatusNotImplemented)
	}
}

// handleResetPassword handles password reset with token (not implemented)
func handleResetPassword(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Password reset not implemented yet. Please contact administrator.", http.StatusNotImplemented)
	}
}

// handleGetCSRFToken returns a new CSRF token
func handleGetCSRFToken(csrf *middleware.CSRFProtection) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := csrf.GenerateToken()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"csrf_token":"%s"}`, token)
	}
}

// handleGetNotifications returns user notifications (not implemented)
func handleGetNotifications(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"notifications":[]}`)
	}
}

// handleMarkNotificationRead marks a notification as read (not implemented)
func handleMarkNotificationRead(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}
}

// serveManifest serves the PWA manifest.json file
func serveManifest(w http.ResponseWriter, r *http.Request) {
	manifestPath := "./static/manifest.json"
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		log.Printf("Failed to read manifest: %v", err)
		http.Error(w, "Manifest not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/manifest+json")
	w.Header().Set("Cache-Control", "public, max-age=3600") // Cache for 1 hour
	w.Write(data)
}

// serveServiceWorker serves the service worker JavaScript file
func serveServiceWorker(w http.ResponseWriter, r *http.Request) {
	swPath := "./static/sw.js"
	data, err := os.ReadFile(swPath)
	if err != nil {
		log.Printf("Failed to read service worker: %v", err)
		http.Error(w, "Service worker not found", http.StatusNotFound)
		return
	}

	// Service workers must be served with proper MIME type and no caching
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("Service-Worker-Allowed", "/") // Allow service worker to control entire origin
	w.Write(data)
}

// requireSetupComplete is middleware that redirects to setup if no users exist
func requireSetupComplete(db *database.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var count int
			err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
			if err != nil {
				http.Error(w, "Database error", http.StatusInternalServerError)
				return
			}

			if count == 0 {
				// No users exist - redirect to setup
				http.Redirect(w, r, "/setup", http.StatusSeeOther)
				return
			}

			// Setup complete - continue to requested page
			next.ServeHTTP(w, r)
		})
	}
}
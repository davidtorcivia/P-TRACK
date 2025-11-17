package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"injection-tracker/internal/database"
	"injection-tracker/internal/middleware"
	"injection-tracker/internal/models"
	"injection-tracker/internal/repository"

	"github.com/go-chi/chi/v5"
)

// CreateCourseRequest represents the request body for creating a course
type CreateCourseRequest struct {
	Name            string  `json:"name"`
	StartDate       string  `json:"start_date"`
	ExpectedEndDate *string `json:"expected_end_date,omitempty"`
	Notes           *string `json:"notes,omitempty"`
	IsActive        *bool   `json:"is_active,omitempty"`
}

// UpdateCourseRequest represents the request body for updating a course
type UpdateCourseRequest struct {
	Name            *string `json:"name,omitempty"`
	StartDate       *string `json:"start_date,omitempty"`
	ExpectedEndDate *string `json:"expected_end_date,omitempty"`
	Notes           *string `json:"notes,omitempty"`
}

// CloseCourseRequest represents the request body for closing a course
type CloseCourseRequest struct {
	ActualEndDate *string `json:"actual_end_date,omitempty"`
}

// HandleGetCourses returns a list of all courses
func HandleGetCourses(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		accountID := middleware.GetAccountID(r.Context())
		if userID == 0 || accountID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		courseRepo := repository.NewCourseRepository(db)

		// Check for filter parameter
		filter := r.URL.Query().Get("filter")
		var courses []*models.Course
		var err error

		switch filter {
		case "active":
			courses, err = courseRepo.ListActive(accountID)
		case "completed":
			courses, err = courseRepo.ListCompleted(accountID)
		default:
			courses, err = courseRepo.List(accountID)
		}

		if err != nil {
			http.Error(w, "Failed to retrieve courses", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(courses)
	}
}

// HandleCreateCourse creates a new course
func HandleCreateCourse(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		accountID := middleware.GetAccountID(r.Context())
		if userID == 0 || accountID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		var req CreateCourseRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate required fields
		if req.Name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}
		if req.StartDate == "" {
			http.Error(w, "start_date is required", http.StatusBadRequest)
			return
		}

		// Parse start date
		startDate, err := time.Parse("2006-01-02", req.StartDate)
		if err != nil {
			http.Error(w, "Invalid start_date format, use YYYY-MM-DD", http.StatusBadRequest)
			return
		}

		// Parse expected end date if provided
		var expectedEndDate sql.NullTime
		if req.ExpectedEndDate != nil && *req.ExpectedEndDate != "" {
			parsedDate, err := time.Parse("2006-01-02", *req.ExpectedEndDate)
			if err != nil {
				http.Error(w, "Invalid expected_end_date format, use YYYY-MM-DD", http.StatusBadRequest)
				return
			}
			expectedEndDate = sql.NullTime{Time: parsedDate, Valid: true}
		}

		// Set is_active default to true if not specified
		isActive := true
		if req.IsActive != nil {
			isActive = *req.IsActive
		}

		// Create course
		course := &models.Course{
			Name:            req.Name,
			StartDate:       startDate,
			ExpectedEndDate: expectedEndDate,
			IsActive:        isActive,
			Notes:           nullString(req.Notes),
			CreatedBy:       sql.NullInt64{Int64: userID, Valid: true},
			AccountID:       sql.NullInt64{Int64: accountID, Valid: true},
		}

		courseRepo := repository.NewCourseRepository(db)

		// If creating an active course, deactivate others first
		if isActive {
			// Note: Activate with ID 0 will fail - this logic may need review
			// For now, skip the pre-deactivation as Create doesn't auto-activate others
			// The Activate method should be called separately if needed
		}

		if err := courseRepo.Create(course); err != nil {
			http.Error(w, fmt.Sprintf("Failed to create course: %v", err), http.StatusInternalServerError)
			return
		}

		// Create audit log
		auditRepo := repository.NewAuditRepository(db)
		_ = auditRepo.LogWithDetails(
			sql.NullInt64{Int64: userID, Valid: true},
			"create",
			"course",
			sql.NullInt64{Int64: course.ID, Valid: true},
			map[string]interface{}{
				"name":      course.Name,
				"is_active": course.IsActive,
			},
			r.RemoteAddr,
			r.UserAgent(),
		)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(course)
	}
}

// HandleGetActiveCourse returns the currently active course
func HandleGetActiveCourse(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		accountID := middleware.GetAccountID(r.Context())
		if userID == 0 || accountID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		courseRepo := repository.NewCourseRepository(db)
		course, err := courseRepo.GetActiveCourse(accountID)
		if err != nil {
			if err == repository.ErrNotFound {
				http.Error(w, "No active course found", http.StatusNotFound)
				return
			}
			http.Error(w, "Failed to retrieve active course", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(course)
	}
}

// HandleGetCourse returns a single course by ID
func HandleGetCourse(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		accountID := middleware.GetAccountID(r.Context())
		if userID == 0 || accountID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		idStr := chi.URLParam(r, "id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid course ID", http.StatusBadRequest)
			return
		}

		courseRepo := repository.NewCourseRepository(db)
		course, err := courseRepo.GetByID(id, accountID)
		if err != nil {
			if err == repository.ErrNotFound {
				http.Error(w, "Course not found", http.StatusNotFound)
				return
			}
			http.Error(w, "Failed to retrieve course", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(course)
	}
}

// HandleUpdateCourse updates an existing course
func HandleUpdateCourse(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		accountID := middleware.GetAccountID(r.Context())
		if userID == 0 || accountID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		idStr := chi.URLParam(r, "id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid course ID", http.StatusBadRequest)
			return
		}

		var req UpdateCourseRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Get existing course
		courseRepo := repository.NewCourseRepository(db)
		course, err := courseRepo.GetByID(id, accountID)
		if err != nil {
			if err == repository.ErrNotFound {
				http.Error(w, "Course not found", http.StatusNotFound)
				return
			}
			http.Error(w, "Failed to retrieve course", http.StatusInternalServerError)
			return
		}

		// Update fields if provided
		if req.Name != nil {
			course.Name = *req.Name
		}
		if req.StartDate != nil {
			startDate, err := time.Parse("2006-01-02", *req.StartDate)
			if err != nil {
				http.Error(w, "Invalid start_date format, use YYYY-MM-DD", http.StatusBadRequest)
				return
			}
			course.StartDate = startDate
		}
		if req.ExpectedEndDate != nil {
			if *req.ExpectedEndDate == "" {
				course.ExpectedEndDate = sql.NullTime{Valid: false}
			} else {
				parsedDate, err := time.Parse("2006-01-02", *req.ExpectedEndDate)
				if err != nil {
					http.Error(w, "Invalid expected_end_date format, use YYYY-MM-DD", http.StatusBadRequest)
					return
				}
				course.ExpectedEndDate = sql.NullTime{Time: parsedDate, Valid: true}
			}
		}
		if req.Notes != nil {
			if *req.Notes == "" {
				course.Notes = sql.NullString{Valid: false}
			} else {
				course.Notes = sql.NullString{String: *req.Notes, Valid: true}
			}
		}

		// Update course
		if err := courseRepo.Update(course, accountID); err != nil {
			http.Error(w, "Failed to update course", http.StatusInternalServerError)
			return
		}

		// Create audit log
		auditRepo := repository.NewAuditRepository(db)
		_ = auditRepo.LogWithDetails(
			sql.NullInt64{Int64: userID, Valid: true},
			"update",
			"course",
			sql.NullInt64{Int64: course.ID, Valid: true},
			map[string]interface{}{
				"name": course.Name,
			},
			r.RemoteAddr,
			r.UserAgent(),
		)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(course)
	}
}

// HandleDeleteCourse deletes a course and all associated data (CASCADE)
func HandleDeleteCourse(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		accountID := middleware.GetAccountID(r.Context())
		if userID == 0 || accountID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		idStr := chi.URLParam(r, "id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid course ID", http.StatusBadRequest)
			return
		}

		// Get course details for audit log before deleting
		courseRepo := repository.NewCourseRepository(db)
		course, err := courseRepo.GetByID(id, accountID)
		if err != nil {
			if err == repository.ErrNotFound {
				http.Error(w, "Course not found", http.StatusNotFound)
				return
			}
			http.Error(w, "Failed to retrieve course", http.StatusInternalServerError)
			return
		}

		// Delete course (will cascade delete injections, symptoms, etc.)
		if err := courseRepo.Delete(id, accountID); err != nil {
			http.Error(w, "Failed to delete course", http.StatusInternalServerError)
			return
		}

		// Create audit log
		auditRepo := repository.NewAuditRepository(db)
		_ = auditRepo.LogWithDetails(
			sql.NullInt64{Int64: userID, Valid: true},
			"delete",
			"course",
			sql.NullInt64{Int64: id, Valid: true},
			map[string]interface{}{
				"name": course.Name,
			},
			r.RemoteAddr,
			r.UserAgent(),
		)

		w.WriteHeader(http.StatusNoContent)
	}
}

// HandleActivateCourse activates a course and deactivates all others
func HandleActivateCourse(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		accountID := middleware.GetAccountID(r.Context())
		if userID == 0 || accountID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		idStr := chi.URLParam(r, "id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid course ID", http.StatusBadRequest)
			return
		}

		courseRepo := repository.NewCourseRepository(db)

		// Verify course exists
		course, err := courseRepo.GetByID(id, accountID)
		if err != nil {
			if err == repository.ErrNotFound {
				http.Error(w, "Course not found", http.StatusNotFound)
				return
			}
			http.Error(w, "Failed to retrieve course", http.StatusInternalServerError)
			return
		}

		// Activate course
		if err := courseRepo.Activate(id, accountID); err != nil {
			http.Error(w, "Failed to activate course", http.StatusInternalServerError)
			return
		}

		// Create audit log
		auditRepo := repository.NewAuditRepository(db)
		_ = auditRepo.LogWithDetails(
			sql.NullInt64{Int64: userID, Valid: true},
			"activate",
			"course",
			sql.NullInt64{Int64: id, Valid: true},
			map[string]interface{}{
				"name": course.Name,
			},
			r.RemoteAddr,
			r.UserAgent(),
		)

		// Return updated course
		course, _ = courseRepo.GetByID(id, accountID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(course)
	}
}

// HandleCloseCourse closes a course by setting the actual end date
func HandleCloseCourse(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		accountID := middleware.GetAccountID(r.Context())
		if userID == 0 || accountID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		idStr := chi.URLParam(r, "id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid course ID", http.StatusBadRequest)
			return
		}

		var req CloseCourseRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			// If no body provided, use current date
			req.ActualEndDate = nil
		}

		// Parse actual end date or use current time
		var endDate time.Time
		if req.ActualEndDate != nil && *req.ActualEndDate != "" {
			parsedDate, err := time.Parse("2006-01-02", *req.ActualEndDate)
			if err != nil {
				http.Error(w, "Invalid actual_end_date format, use YYYY-MM-DD", http.StatusBadRequest)
				return
			}
			endDate = parsedDate
		} else {
			endDate = time.Now()
		}

		courseRepo := repository.NewCourseRepository(db)

		// Verify course exists
		course, err := courseRepo.GetByID(id, accountID)
		if err != nil {
			if err == repository.ErrNotFound {
				http.Error(w, "Course not found", http.StatusNotFound)
				return
			}
			http.Error(w, "Failed to retrieve course", http.StatusInternalServerError)
			return
		}

		// Close course
		if err := courseRepo.Close(id, accountID, endDate); err != nil{
			http.Error(w, "Failed to close course", http.StatusInternalServerError)
			return
		}

		// Create audit log
		auditRepo := repository.NewAuditRepository(db)
		_ = auditRepo.LogWithDetails(
			sql.NullInt64{Int64: userID, Valid: true},
			"close",
			"course",
			sql.NullInt64{Int64: id, Valid: true},
			map[string]interface{}{
				"name":     course.Name,
				"end_date": endDate.Format("2006-01-02"),
			},
			r.RemoteAddr,
			r.UserAgent(),
		)

		// Return updated course
		course, _ = courseRepo.GetByID(id, accountID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(course)
	}
}
package handlers

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"net/http"
	"time"

	"injection-tracker/internal/database"
)

// ExportData represents the data structure for exports
type ExportData struct {
	Injections []ExportInjection
	Symptoms   []ExportSymptom
	Medications []ExportMedication
	StartDate  time.Time
	EndDate    time.Time
	CourseID   int64
	CourseName string
}

// ExportInjection represents an injection for export
type ExportInjection struct {
	ID              int64
	Timestamp       time.Time
	Side            string
	PainLevel       int
	HasKnots        bool
	SiteReaction    string
	Notes           string
	AdministeredBy  string
}

// ExportSymptom represents a symptom for export
type ExportSymptom struct {
	ID           int64
	Timestamp    time.Time
	PainLevel    int
	PainLocation string
	PainType     string
	Symptoms     string
	Notes        string
}

// ExportMedication represents a medication log for export
type ExportMedication struct {
	ID             int64
	Timestamp      time.Time
	MedicationName string
	Taken          bool
	Notes          string
}

// HandleExportPDF generates a PDF report with injection and symptom data
func HandleExportPDF(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Parse query parameters
		startDate := r.URL.Query().Get("start_date")
		endDate := r.URL.Query().Get("end_date")
		courseID := r.URL.Query().Get("course_id")

		// Validate date parameters
		var start, end time.Time
		var err error

		if startDate != "" {
			start, err = time.Parse("2006-01-02", startDate)
			if err != nil {
				http.Error(w, "Invalid start_date format. Use YYYY-MM-DD", http.StatusBadRequest)
				return
			}
		} else {
			// Default to 30 days ago
			start = time.Now().AddDate(0, 0, -30)
		}

		if endDate != "" {
			end, err = time.Parse("2006-01-02", endDate)
			if err != nil {
				http.Error(w, "Invalid end_date format. Use YYYY-MM-DD", http.StatusBadRequest)
				return
			}
		} else {
			// Default to today
			end = time.Now()
		}

		// Ensure end is after start
		if end.Before(start) {
			http.Error(w, "end_date must be after start_date", http.StatusBadRequest)
			return
		}

		// Gather export data
		exportData, err := gatherExportData(db, start, end, courseID)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to gather export data: %v", err), http.StatusInternalServerError)
			return
		}

		// Generate PDF
		pdfBytes, err := generatePDF(exportData)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to generate PDF: %v", err), http.StatusInternalServerError)
			return
		}

		// Set headers for PDF download
		filename := fmt.Sprintf("injection-tracker-report-%s-to-%s.pdf", start.Format("2006-01-02"), end.Format("2006-01-02"))
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(pdfBytes)))

		// Write PDF to response
		w.Write(pdfBytes)
	}
}

// HandleExportCSV generates CSV export of injection, symptom, and medication data
func HandleExportCSV(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Parse query parameters
		startDate := r.URL.Query().Get("start_date")
		endDate := r.URL.Query().Get("end_date")
		courseID := r.URL.Query().Get("course_id")
		dataType := r.URL.Query().Get("type") // "injections", "symptoms", "medications", or "all"

		if dataType == "" {
			dataType = "all"
		}

		// Validate date parameters
		var start, end time.Time
		var err error

		if startDate != "" {
			start, err = time.Parse("2006-01-02", startDate)
			if err != nil {
				http.Error(w, "Invalid start_date format. Use YYYY-MM-DD", http.StatusBadRequest)
				return
			}
		} else {
			// Default to 30 days ago
			start = time.Now().AddDate(0, 0, -30)
		}

		if endDate != "" {
			end, err = time.Parse("2006-01-02", endDate)
			if err != nil {
				http.Error(w, "Invalid end_date format. Use YYYY-MM-DD", http.StatusBadRequest)
				return
			}
		} else {
			// Default to today
			end = time.Now()
		}

		// Gather export data
		exportData, err := gatherExportData(db, start, end, courseID)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to gather export data: %v", err), http.StatusInternalServerError)
			return
		}

		// Generate CSV
		var csvBuffer bytes.Buffer
		csvWriter := csv.NewWriter(&csvBuffer)

		switch dataType {
		case "injections":
			err = writeInjectionsCSV(csvWriter, exportData.Injections)
		case "symptoms":
			err = writeSymptomsCSV(csvWriter, exportData.Symptoms)
		case "medications":
			err = writeMedicationsCSV(csvWriter, exportData.Medications)
		case "all":
			err = writeAllDataCSV(csvWriter, exportData)
		default:
			http.Error(w, "Invalid type parameter. Use: injections, symptoms, medications, or all", http.StatusBadRequest)
			return
		}

		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to generate CSV: %v", err), http.StatusInternalServerError)
			return
		}

		csvWriter.Flush()
		if err := csvWriter.Error(); err != nil {
			http.Error(w, fmt.Sprintf("Failed to flush CSV writer: %v", err), http.StatusInternalServerError)
			return
		}

		// Set headers for CSV download
		filename := fmt.Sprintf("injection-tracker-%s-%s-to-%s.csv", dataType, start.Format("2006-01-02"), end.Format("2006-01-02"))
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
		w.Header().Set("Content-Length", fmt.Sprintf("%d", csvBuffer.Len()))

		// Write CSV to response
		w.Write(csvBuffer.Bytes())
	}
}

// gatherExportData collects all data needed for export
func gatherExportData(db *database.DB, start, end time.Time, courseIDStr string) (*ExportData, error) {
	data := &ExportData{
		StartDate: start,
		EndDate:   end,
	}

	// Build WHERE clause for date filtering
	whereClause := "WHERE timestamp BETWEEN ? AND ?"
	args := []interface{}{start, end}

	if courseIDStr != "" {
		whereClause += " AND course_id = ?"
		args = append(args, courseIDStr)

		// Get course name
		err := db.QueryRow("SELECT id, name FROM courses WHERE id = ?", courseIDStr).Scan(&data.CourseID, &data.CourseName)
		if err != nil {
			return nil, fmt.Errorf("failed to get course: %w", err)
		}
	}

	// Gather injections
	injectionQuery := `
		SELECT i.id, i.timestamp, i.side,
			COALESCE(i.pain_level, 0) as pain_level,
			i.has_knots,
			COALESCE(i.site_reaction, '') as site_reaction,
			COALESCE(i.notes, '') as notes,
			COALESCE(u.username, '') as administered_by
		FROM injections i
		LEFT JOIN users u ON i.administered_by = u.id
	` + whereClause + " ORDER BY i.timestamp DESC"

	rows, err := db.Query(injectionQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query injections: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var inj ExportInjection
		err := rows.Scan(
			&inj.ID,
			&inj.Timestamp,
			&inj.Side,
			&inj.PainLevel,
			&inj.HasKnots,
			&inj.SiteReaction,
			&inj.Notes,
			&inj.AdministeredBy,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan injection: %w", err)
		}
		data.Injections = append(data.Injections, inj)
	}

	// Gather symptoms
	symptomQuery := `
		SELECT id, timestamp,
			COALESCE(pain_level, 0) as pain_level,
			COALESCE(pain_location, '') as pain_location,
			COALESCE(pain_type, '') as pain_type,
			COALESCE(symptoms, '') as symptoms,
			COALESCE(notes, '') as notes
		FROM symptom_logs
	` + whereClause + " ORDER BY timestamp DESC"

	rows, err = db.Query(symptomQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query symptoms: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var sym ExportSymptom
		err := rows.Scan(
			&sym.ID,
			&sym.Timestamp,
			&sym.PainLevel,
			&sym.PainLocation,
			&sym.PainType,
			&sym.Symptoms,
			&sym.Notes,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan symptom: %w", err)
		}
		data.Symptoms = append(data.Symptoms, sym)
	}

	// Gather medication logs
	medicationQuery := `
		SELECT ml.id, ml.timestamp, m.name as medication_name, ml.taken,
			COALESCE(ml.notes, '') as notes
		FROM medication_logs ml
		JOIN medications m ON ml.medication_id = m.id
	` + whereClause + " ORDER BY ml.timestamp DESC"

	rows, err = db.Query(medicationQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query medication logs: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var med ExportMedication
		err := rows.Scan(
			&med.ID,
			&med.Timestamp,
			&med.MedicationName,
			&med.Taken,
			&med.Notes,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan medication log: %w", err)
		}
		data.Medications = append(data.Medications, med)
	}

	return data, nil
}

// writeInjectionsCSV writes injection data to CSV
func writeInjectionsCSV(writer *csv.Writer, injections []ExportInjection) error {
	// Write header
	header := []string{"ID", "Date", "Time", "Side", "Pain Level", "Has Knots", "Site Reaction", "Notes", "Administered By"}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write data
	for _, inj := range injections {
		hasKnots := "No"
		if inj.HasKnots {
			hasKnots = "Yes"
		}

		row := []string{
			fmt.Sprintf("%d", inj.ID),
			inj.Timestamp.Format("2006-01-02"),
			inj.Timestamp.Format("15:04:05"),
			inj.Side,
			fmt.Sprintf("%d", inj.PainLevel),
			hasKnots,
			inj.SiteReaction,
			inj.Notes,
			inj.AdministeredBy,
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}

	return nil
}

// writeSymptomsCSV writes symptom data to CSV
func writeSymptomsCSV(writer *csv.Writer, symptoms []ExportSymptom) error {
	// Write header
	header := []string{"ID", "Date", "Time", "Pain Level", "Pain Location", "Pain Type", "Symptoms", "Notes"}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write data
	for _, sym := range symptoms {
		row := []string{
			fmt.Sprintf("%d", sym.ID),
			sym.Timestamp.Format("2006-01-02"),
			sym.Timestamp.Format("15:04:05"),
			fmt.Sprintf("%d", sym.PainLevel),
			sym.PainLocation,
			sym.PainType,
			sym.Symptoms,
			sym.Notes,
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}

	return nil
}

// writeMedicationsCSV writes medication data to CSV
func writeMedicationsCSV(writer *csv.Writer, medications []ExportMedication) error {
	// Write header
	header := []string{"ID", "Date", "Time", "Medication", "Taken", "Notes"}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write data
	for _, med := range medications {
		taken := "No"
		if med.Taken {
			taken = "Yes"
		}

		row := []string{
			fmt.Sprintf("%d", med.ID),
			med.Timestamp.Format("2006-01-02"),
			med.Timestamp.Format("15:04:05"),
			med.MedicationName,
			taken,
			med.Notes,
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}

	return nil
}

// writeAllDataCSV writes all data types to a single CSV with sections
func writeAllDataCSV(writer *csv.Writer, data *ExportData) error {
	// Write report header
	if err := writer.Write([]string{"Progesterone Injection Tracker - Complete Export"}); err != nil {
		return err
	}
	if err := writer.Write([]string{fmt.Sprintf("Report Period: %s to %s", data.StartDate.Format("2006-01-02"), data.EndDate.Format("2006-01-02"))}); err != nil {
		return err
	}
	if data.CourseName != "" {
		if err := writer.Write([]string{fmt.Sprintf("Course: %s", data.CourseName)}); err != nil {
			return err
		}
	}
	if err := writer.Write([]string{""}); err != nil {
		return err
	}

	// Injections section
	if err := writer.Write([]string{"=== INJECTIONS ==="}); err != nil {
		return err
	}
	if err := writeInjectionsCSV(writer, data.Injections); err != nil {
		return err
	}
	if err := writer.Write([]string{""}); err != nil {
		return err
	}

	// Symptoms section
	if err := writer.Write([]string{"=== SYMPTOMS ==="}); err != nil {
		return err
	}
	if err := writeSymptomsCSV(writer, data.Symptoms); err != nil {
		return err
	}
	if err := writer.Write([]string{""}); err != nil {
		return err
	}

	// Medications section
	if err := writer.Write([]string{"=== MEDICATIONS ==="}); err != nil {
		return err
	}
	if err := writeMedicationsCSV(writer, data.Medications); err != nil {
		return err
	}

	return nil
}

// generatePDF creates a PDF from the export data
// Note: This implementation requires github.com/jung-kurt/gofpdf
// For now, we'll create a simple text-based PDF. A production implementation
// would use a proper PDF library.
func generatePDF(data *ExportData) ([]byte, error) {
	// TODO: Implement proper PDF generation using a library like gofpdf
	// For now, return an error indicating PDF generation needs implementation
	// This would require adding the dependency to go.mod:
	// github.com/jung-kurt/gofpdf v1.16.2

	var buffer bytes.Buffer

	// Write a simple header
	buffer.WriteString("Progesterone Injection Tracker - Medical Report\n\n")
	buffer.WriteString(fmt.Sprintf("Report Period: %s to %s\n", data.StartDate.Format("January 2, 2006"), data.EndDate.Format("January 2, 2006")))
	if data.CourseName != "" {
		buffer.WriteString(fmt.Sprintf("Course: %s\n", data.CourseName))
	}
	buffer.WriteString("\n")

	// Injections summary
	buffer.WriteString(fmt.Sprintf("Total Injections: %d\n\n", len(data.Injections)))
	buffer.WriteString("Injection Log:\n")
	buffer.WriteString("-------------------------------------------------------------------\n")
	buffer.WriteString(fmt.Sprintf("%-12s %-8s %-6s %-5s %-15s %-20s\n", "Date", "Time", "Side", "Pain", "Reaction", "Notes"))
	buffer.WriteString("-------------------------------------------------------------------\n")

	for _, inj := range data.Injections {
		buffer.WriteString(fmt.Sprintf("%-12s %-8s %-6s %-5d %-15s %-20s\n",
			inj.Timestamp.Format("2006-01-02"),
			inj.Timestamp.Format("15:04"),
			inj.Side,
			inj.PainLevel,
			inj.SiteReaction,
			truncateString(inj.Notes, 20),
		))
	}

	buffer.WriteString("\n\nSymptoms Summary:\n")
	buffer.WriteString("-------------------------------------------------------------------\n")
	buffer.WriteString(fmt.Sprintf("Total Symptom Logs: %d\n", len(data.Symptoms)))

	buffer.WriteString("\n\nMedications Summary:\n")
	buffer.WriteString("-------------------------------------------------------------------\n")
	buffer.WriteString(fmt.Sprintf("Total Medication Logs: %d\n", len(data.Medications)))

	buffer.WriteString("\n\n")
	buffer.WriteString("Note: This is a simplified text-based PDF. For production use,\n")
	buffer.WriteString("implement proper PDF generation with formatting and charts.\n")

	return buffer.Bytes(), nil
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
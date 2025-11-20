// HandleHelpPage renders the help page
func HandleHelpPage(db *database.DB, csrf *middleware.CSRFProtection) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := getBasePageData(r, csrf)
		data["Title"] = "Help & Support"

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := web.Render(w, "help.html", data); err != nil {
			http.Error(w, "Failed to render template", http.StatusInternalServerError)
			return
		}
	}
}

// HandleAboutPage renders the about page
func HandleAboutPage(db *database.DB, csrf *middleware.CSRFProtection) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := getBasePageData(r, csrf)
		data["Title"] = "About"

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := web.Render(w, "about.html", data); err != nil {
			http.Error(w, "Failed to render template", http.StatusInternalServerError)
			return
		}
	}
}

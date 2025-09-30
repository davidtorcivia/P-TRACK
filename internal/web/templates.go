package web

import (
	"fmt"
	"html/template"
	"io"
	"path/filepath"
)

var templates map[string]*template.Template

// InitTemplates loads all HTML templates
func InitTemplates() error {
	templates = make(map[string]*template.Template)

	// Define helper functions
	funcMap := template.FuncMap{
		"formatDate":     formatDate,
		"formatDateTime": formatDateTime,
		"formatTime":     formatTime,
		"sideBadgeClass": sideBadgeClass,
		"painLevelClass": painLevelClass,
		"painLevelEmoji": painLevelEmoji,
		"timeAgo":        timeAgo,
	}

	// Get all page templates
	pages, err := filepath.Glob(filepath.Join("templates", "pages", "*.html"))
	if err != nil {
		return err
	}

	// Parse each page with its layout
	for _, page := range pages {
		// Get the page name (e.g., "login.html")
		pageName := filepath.Base(page)

		// Parse the base layout, the specific page, and any components together
		tmpl := template.New(pageName).Funcs(funcMap)

		// Parse base layout first
		tmpl, err = tmpl.ParseFiles(filepath.Join("templates", "layouts", "base.html"))
		if err != nil {
			return err
		}

		// Parse the specific page
		tmpl, err = tmpl.ParseFiles(page)
		if err != nil {
			return err
		}

		// Parse components if they exist
		components, _ := filepath.Glob(filepath.Join("templates", "components", "*.html"))
		if len(components) > 0 {
			tmpl, err = tmpl.ParseFiles(components...)
			if err != nil {
				return err
			}
		}

		// Store the template with the base layout as the entry point
		templates[pageName] = tmpl
	}

	return nil
}

// Render renders a template with data
// The name should be the page template name (e.g., "login.html")
// This will execute base.html which includes the page's content block
func Render(w io.Writer, name string, data interface{}) error {
	tmpl, ok := templates[name]
	if !ok {
		return fmt.Errorf("template not found: %s", name)
	}
	// Execute base.html which will include the content block from the page
	return tmpl.ExecuteTemplate(w, "base.html", data)
}
#!/bin/bash
# Patch the registration success response

FILE="/home/user/P-TRACK/internal/handlers/auth_handlers.go"
TEMP="/tmp/auth_handlers_patched.go"

# Use awk to replace lines 511-513 with success message
awk '
/HTMX request - redirect to login page/ {
    print "\t\t// HTMX request - show success message then redirect"
    print "\t\tw.Header().Set(\"Content-Type\", \"text/html\")"
    print "\t\tw.WriteHeader(http.StatusOK)"
    print "\t\tsuccessHTML := `"
    print "<div role=\"alert\" style=\""
    print "\tbackground-color: #d4edda;"
    print "\tborder: 2px solid #28a745;"
    print "\tborder-radius: 8px;"
    print "\tpadding: 1rem 1.25rem;"
    print "\tmargin-bottom: 1.5rem;"
    print "\tbox-shadow: 0 2px 4px rgba(0,0,0,0.1);"
    print "\">"
    print "\t<div style=\"display: flex; align-items: start; gap: 0.75rem;\">"
    print "\t\t<span style=\"font-size: 1.5rem; line-height: 1;\">✓</span>"
    print "\t\t<div style=\"flex: 1;\">"
    print "\t\t\t<strong style=\"color: #28a745; font-size: 1rem; display: block; margin-bottom: 0.25rem;\">Success!</strong>"
    print "\t\t\t<p style=\"color: #155724; margin: 0; font-size: 0.95rem; line-height: 1.5;\">"
    print "\t\t\t\tAccount created successfully. Redirecting to login..."
    print "\t\t\t</p>"
    print "\t\t</div>"
    print "\t</div>"
    print "</div>"
    print "<script>"
    print "\tsetTimeout(function() {"
    print "\t\twindow.location.href = \"/login?registered=true\";"
    print "\t}, 1500);"
    print "</script>`"
    print "\t\tfmt.Fprint(w, successHTML)"
    getline  # skip "w.Header().Set("HX-Redirect", "/login?registered=true")"
    getline  # skip "w.WriteHeader(http.StatusOK)"
    next
}
{ print }
' "$FILE" > "$TEMP"

mv "$TEMP" "$FILE"
echo "Patched registration success response"

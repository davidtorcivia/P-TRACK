package web

import (
	"fmt"
	"time"
)

// formatDate formats a date as YYYY-MM-DD
func formatDate(t time.Time) string {
	return t.Format("2006-01-02")
}

// formatDateTime formats a datetime as YYYY-MM-DD HH:MM
func formatDateTime(t time.Time) string {
	return t.Format("2006-01-02 15:04")
}

// formatTime formats a time as HH:MM
func formatTime(t time.Time) string {
	return t.Format("15:04")
}

// sideBadgeClass returns CSS class for injection side badge
func sideBadgeClass(side string) string {
	if side == "left" {
		return "badge-left"
	}
	return "badge-right"
}

// painLevelClass returns CSS class for pain level indicator
func painLevelClass(level int) string {
	if level <= 3 {
		return "pain-low"
	} else if level <= 6 {
		return "pain-medium"
	}
	return "pain-high"
}

// painLevelEmoji returns emoji for pain level
func painLevelEmoji(level int) string {
	if level <= 2 {
		return "😊"
	} else if level <= 4 {
		return "🙂"
	} else if level <= 6 {
		return "😐"
	} else if level <= 8 {
		return "😣"
	}
	return "😫"
}

// timeAgo returns human-readable time difference
func timeAgo(t time.Time) string {
	duration := time.Since(t)

	if duration < time.Minute {
		return "just now"
	} else if duration < time.Hour {
		minutes := int(duration.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	} else if duration < 24*time.Hour {
		hours := int(duration.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	} else if duration < 7*24*time.Hour {
		days := int(duration.Hours() / 24)
		if days == 1 {
			return "yesterday"
		}
		return fmt.Sprintf("%d days ago", days)
	} else if duration < 30*24*time.Hour {
		weeks := int(duration.Hours() / 24 / 7)
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	}

	return formatDate(t)
}
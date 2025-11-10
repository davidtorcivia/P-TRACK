# Remaining Work Summary

## Completed ✅
1. Fixed symptom location CHECK constraint (migration 003)
2. Added knots tracking checkbox and field
3. Fixed medication delete to actually delete (hard delete)
4. Added styled confirmation dialogs everywhere
5. Improved medications UI with better frequency system
6. Added expiration dates to inventory
7. Improved settings page layout

## Still To Do ❌

### 1. Theme Settings Implementation
The UI exists but needs backend support:
- Add `theme` field to user settings (or separate user_preferences table)
- Create handler for `/api/settings/app` POST
- Store theme preference (light/dark/auto)
- Pass theme to base template
- Add JavaScript to apply theme on load

### 2. Timezone Settings Implementation
The UI exists but needs full backend implementation:
- Add `timezone` field to user settings
- Default to 'America/New_York'
- Convert ALL timestamps when displaying:
  - Dashboard
  - Injections list
  - Symptoms
  - Medications logs
  - Activity feed
  - Inventory history
- Use Go's `time.LoadLocation()` and `time.In()` for conversion
- DST is handled automatically by Go's time package

### 3. Medications Boolean Display
Need to find where `{test true}` or similar is still showing up.
Likely in:
- Dashboard recent activity?
- Medications page template?
- Need to trace through template rendering

## Implementation Notes

### Theme Implementation
```go
// In settings handler, add theme field
type AppSettingsRequest struct {
    Theme         *string `json:"theme"`
    Timezone      *string `json:"timezone"`
    DateFormat    *string `json:"date_format"`
    TimeFormat    *string `json:"time_format"`
    AdvancedMode  *bool   `json:"advanced_mode"`
}

// Store in settings table with key 'user_theme_{userID}'
// Or better: create user_preferences table
```

### Timezone Implementation
```go
// Helper function to convert timestamps
func convertToUserTimezone(t time.Time, timezone string) time.Time {
    loc, err := time.LoadLocation(timezone)
    if err != nil {
        loc = time.UTC
    }
    return t.In(loc)
}

// Use in all handlers that return timestamps
// Format with user's date/time format preferences
```

### Quick Wins
1. Run the app with `./injection-tracker.exe` to test migration
2. Try adding a symptom with new locations to verify migration worked
3. Try deleting a medication to verify hard delete works
4. Check medications page for any boolean display issues

## Priority
1. **HIGH**: Test that existing changes work (especially migration)
2. **HIGH**: Find and fix medications boolean display
3. **MEDIUM**: Implement theme switching (relatively simple)
4. **MEDIUM**: Implement timezone (requires touching many handlers)

## Testing Checklist
- [ ] Migration 003 runs successfully
- [ ] Can add symptoms with new location values
- [ ] Knots checkbox appears and saves
- [ ] Delete medication actually removes it
- [ ] No `{test true}` or boolean values visible anywhere
- [ ] Theme selector changes theme
- [ ] Timezone selector adjusts all displayed times

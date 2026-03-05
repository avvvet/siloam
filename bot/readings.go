package bot

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/avvvet/siloam/db"
)

// Units A to P
var allUnits = []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p"}

// parseReadings parses "a=340, b=590" style input
// Returns map of unit -> value
func parseReadings(text string) map[string]int {
	result := make(map[string]int)
	re := regexp.MustCompile(`(?i)([a-p])\s*=\s*(\d+)`)
	matches := re.FindAllStringSubmatch(text, -1)
	for _, m := range matches {
		unit := strings.ToLower(m[1])
		val, err := strconv.Atoi(m[2])
		if err == nil {
			result[unit] = val
		}
	}
	return result
}

// isSubmissionOpen returns true if current time is between 6th 00:00 and 7th 00:00 GMT+3
func isSubmissionOpen() bool {
	loc, _ := time.LoadLocation("Europe/Moscow") // GMT+3
	now := time.Now().In(loc)
	day := now.Day()
	return day == 6
}

// isBeforeWindow returns true if before the 6th
func isBeforeWindow() bool {
	loc, _ := time.LoadLocation("Europe/Moscow")
	now := time.Now().In(loc)
	return now.Day() < 6
}

// nextSixth returns the next 6th date string
func nextSixth() string {
	loc, _ := time.LoadLocation("Europe/Moscow")
	now := time.Now().In(loc)
	next := time.Date(now.Year(), now.Month()+1, 6, 0, 0, 0, 0, loc)
	if now.Day() < 6 {
		next = time.Date(now.Year(), now.Month(), 6, 0, 0, 0, 0, loc)
	}
	return next.Format("January 6, 2006")
}

// buildSummary builds the full summary message
func buildSummary(readings map[string]*db.Reading) string {
	loc, _ := time.LoadLocation("Europe/Moscow")
	now := time.Now().In(loc)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📋 *Water Readings — %s*\n\n", now.Format("January 2006")))

	submitted := 0
	for _, unit := range allUnits {
		r, ok := readings[unit]
		if ok {
			if r.Updated {
				sb.WriteString(fmt.Sprintf("🔄 *%s:* %d _(updated from %d)_\n", strings.ToUpper(unit), r.Value, r.OldValue))
			} else {
				sb.WriteString(fmt.Sprintf("✅ *%s:* %d\n", strings.ToUpper(unit), r.Value))
			}
			submitted++
		} else {
			sb.WriteString(fmt.Sprintf("❌ *%s:* not submitted\n", strings.ToUpper(unit)))
		}
	}

	sb.WriteString(fmt.Sprintf("\n*Submitted: %d/16 | Pending: %d*", submitted, 16-submitted))
	return sb.String()
}

// pendingUnits returns list of units not yet submitted
func pendingUnits(readings map[string]*db.Reading) []string {
	var pending []string
	for _, unit := range allUnits {
		if _, ok := readings[unit]; !ok {
			pending = append(pending, strings.ToUpper(unit))
		}
	}
	sort.Strings(pending)
	return pending
}

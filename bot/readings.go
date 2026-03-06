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

// parsePayments parses "a=300birr, b=450birr" style input
func parsePayments(text string) map[string]float64 {
	result := make(map[string]float64)
	re := regexp.MustCompile(`(?i)([a-p])\s*=\s*(\d+(?:\.\d+)?)\s*birr`)
	matches := re.FindAllStringSubmatch(text, -1)
	for _, m := range matches {
		unit := strings.ToLower(m[1])
		val, err := strconv.ParseFloat(m[2], 64)
		if err == nil {
			result[unit] = val
		}
	}
	return result
}

// parseBillCommand parses "/bill 5000" and returns amount
func parseBillCommand(text string) (float64, bool) {
	re := regexp.MustCompile(`(?i)^/bill\s+(\d+(?:\.\d+)?)`)
	m := re.FindStringSubmatch(strings.TrimSpace(text))
	if m == nil {
		return 0, false
	}
	val, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, false
	}
	return val, true
}

// isSubmissionOpen returns true if current time is on the 6th GMT+3
func isSubmissionOpen() bool {
	loc, _ := time.LoadLocation("Europe/Moscow")
	now := time.Now().In(loc)
	return now.Day() == 6
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

// allSubmitted returns true if all 16 units have submitted
func allSubmitted(readings map[string]*db.Reading) bool {
	return len(readings) == 16
}

// buildSummary builds the full reading summary message
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

// buildPaymentSummary builds the payment status summary
func buildPaymentSummary(payments map[string]*db.Payment, bills map[string]float64) string {
	loc, _ := time.LoadLocation("Europe/Moscow")
	now := time.Now().In(loc)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("💳 *Payment Status — %s*\n\n", now.Format("January 2006")))

	var totalPaid, totalOwed float64
	paid := 0

	for _, unit := range allUnits {
		owed := bills[unit]
		totalOwed += owed
		p, ok := payments[unit]
		if ok {
			totalPaid += p.Amount
			paid++
			if p.Updated {
				sb.WriteString(fmt.Sprintf("🔄 *%s:* %.0f Birr _(updated from %.0f)_ ✅\n", strings.ToUpper(unit), p.Amount, p.OldAmount))
			} else {
				sb.WriteString(fmt.Sprintf("✅ *%s:* %.0f Birr\n", strings.ToUpper(unit), p.Amount))
			}
		} else {
			if owed == 0 {
				sb.WriteString(fmt.Sprintf("⬜ *%s:* 0 Birr _(vacant)_\n", strings.ToUpper(unit)))
				paid++ // vacant units count as settled
			} else {
				sb.WriteString(fmt.Sprintf("❌ *%s:* owes %.0f Birr\n", strings.ToUpper(unit), owed))
			}
		}
	}

	sb.WriteString(fmt.Sprintf("\n💰 *Total Paid: %.0f Birr | Remaining: %.0f Birr*", totalPaid, totalOwed-totalPaid))
	sb.WriteString(fmt.Sprintf("\n*Paid: %d/16 | Pending: %d*", paid, 16-paid))
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

// pendingPaymentUnits returns list of units that have not paid
func pendingPaymentUnits(payments map[string]*db.Payment, bills map[string]float64) []string {
	var pending []string
	for _, unit := range allUnits {
		if bills[unit] == 0 {
			continue // vacant, skip
		}
		if _, ok := payments[unit]; !ok {
			pending = append(pending, strings.ToUpper(unit))
		}
	}
	sort.Strings(pending)
	return pending
}

// allPaid returns true if all non-vacant units have paid
func allPaid(payments map[string]*db.Payment, bills map[string]float64) bool {
	return len(pendingPaymentUnits(payments, bills)) == 0
}

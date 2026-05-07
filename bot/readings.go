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

// looksLikePayment returns true if text looks like "a=300" but missing birr
func looksLikePayment(text string) bool {
	re := regexp.MustCompile(`(?i)[a-p]\s*=\s*\d+`)
	return re.MatchString(text)
}

// parseFixReading parses "/fixreading a=340, b=590" style input
func parseFixReading(text string) (map[string]int, bool) {
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(text)), "/fixreading") {
		return nil, false
	}
	// Remove the command prefix
	text = strings.TrimSpace(text[len("/fixreading"):])
	result := parseReadings(text)
	if len(result) == 0 {
		return nil, false
	}
	return result, true
}

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

// buildSummary builds the compact reading summary message
func buildSummary(readings map[string]*db.Reading) string {
	loc, _ := time.LoadLocation("Europe/Moscow")
	now := time.Now().In(loc)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📋 *Water Readings — %s*\n\n", now.Format("January 2006")))

	var submittedList, updatedList, pendingList []string

	for _, unit := range allUnits {
		r, ok := readings[unit]
		if ok {
			if r.Updated {
				updatedList = append(updatedList, fmt.Sprintf("%s:%d→%d", strings.ToUpper(unit), r.OldValue, r.Value))
			} else {
				submittedList = append(submittedList, fmt.Sprintf("%s:%d", strings.ToUpper(unit), r.Value))
			}
		} else {
			pendingList = append(pendingList, strings.ToUpper(unit))
		}
	}

	// Submitted — 4 per row
	if len(submittedList) > 0 {
		sb.WriteString("✅ *Submitted:*\n")
		for i, entry := range submittedList {
			sb.WriteString(fmt.Sprintf("%-12s", entry))
			if (i+1)%4 == 0 {
				sb.WriteString("\n")
			}
		}
		if len(submittedList)%4 != 0 {
			sb.WriteString("\n")
		}
	}

	// Updated — 4 per row
	if len(updatedList) > 0 {
		sb.WriteString("\n🔄 *Updated:*\n")
		for i, entry := range updatedList {
			sb.WriteString(fmt.Sprintf("%-16s", entry))
			if (i+1)%4 == 0 {
				sb.WriteString("\n")
			}
		}
		if len(updatedList)%4 != 0 {
			sb.WriteString("\n")
		}
	}

	// Pending
	if len(pendingList) > 0 {
		sb.WriteString(fmt.Sprintf("\n❌ *Pending:* %s\n", strings.Join(pendingList, ", ")))
	}

	submitted := len(submittedList) + len(updatedList)
	sb.WriteString(fmt.Sprintf("\n*Submitted: %d/16 | Pending: %d*", submitted, 16-submitted))
	return sb.String()
}

// buildPaymentSummary builds the compact payment status summary
func buildPaymentSummary(payments map[string]*db.Payment, bills map[string]float64, additionalFee float64) string {
	loc, _ := time.LoadLocation("Europe/Moscow")
	now := time.Now().In(loc)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("💳 *Payment Status — %s*\n\n", now.Format("January 2006")))

	var paidList, updatedList, pendingList []string
	var totalPaid, totalOwed float64
	paid := 0

	for _, unit := range allUnits {
		waterShare := bills[unit]
		owed := waterShare + additionalFee
		totalOwed += owed
		p, ok := payments[unit]
		if ok {
			totalPaid += p.Amount
			paid++
			if p.Updated {
				updatedList = append(updatedList, fmt.Sprintf("%s:%.0f→%.0f", strings.ToUpper(unit), p.OldAmount, p.Amount))
			} else {
				paidList = append(paidList, fmt.Sprintf("%s:%.0f", strings.ToUpper(unit), p.Amount))
			}
		} else {
			if waterShare == 0 {
				pendingList = append(pendingList, fmt.Sprintf("%s:%.0f(add)", strings.ToUpper(unit), additionalFee))
			} else {
				pendingList = append(pendingList, fmt.Sprintf("%s:%.0f", strings.ToUpper(unit), owed))
			}
		}
	}

	// Paid — 4 per row
	if len(paidList) > 0 {
		sb.WriteString("✅ *Paid:*\n")
		for i, entry := range paidList {
			sb.WriteString(fmt.Sprintf("%-12s", entry))
			if (i+1)%4 == 0 {
				sb.WriteString("\n")
			}
		}
		if len(paidList)%4 != 0 {
			sb.WriteString("\n")
		}
	}

	// Updated — 4 per row
	if len(updatedList) > 0 {
		sb.WriteString("\n🔄 *Updated:*\n")
		for i, entry := range updatedList {
			sb.WriteString(fmt.Sprintf("%-16s", entry))
			if (i+1)%4 == 0 {
				sb.WriteString("\n")
			}
		}
		if len(updatedList)%4 != 0 {
			sb.WriteString("\n")
		}
	}

	// Not paid — 4 per row
	if len(pendingList) > 0 {
		sb.WriteString("\n❌ *Not paid:*\n")
		for i, entry := range pendingList {
			sb.WriteString(fmt.Sprintf("%-14s", entry))
			if (i+1)%4 == 0 {
				sb.WriteString("\n")
			}
		}
		if len(pendingList)%4 != 0 {
			sb.WriteString("\n")
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
func pendingPaymentUnits(payments map[string]*db.Payment, bills map[string]float64, additionalFee float64) []string {
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
func allPaid(payments map[string]*db.Payment, bills map[string]float64, additionalFee float64) bool {
	return len(pendingPaymentUnits(payments, bills, additionalFee)) == 0
}

package tahor

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/avvvet/siloam/db"
)

var allUnits = []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p"}

const fundAmount = 600.0

// parseTahorReset parses "tahor reset a" format
func parseTahorReset(text string) (string, bool) {
	re := regexp.MustCompile(`(?i)^tahor\s+reset\s+([a-p])`)
	m := re.FindStringSubmatch(strings.TrimSpace(text))
	if m == nil {
		return "", false
	}
	return strings.ToLower(m[1]), true
}

// parseTahorPayment parses "tahor a=600" format
func parseTahorPayment(text string) (string, float64, bool) {
	re := regexp.MustCompile(`(?i)^tahor\s+([a-p])\s*=\s*(\d+(?:\.\d+)?)`)
	m := re.FindStringSubmatch(strings.TrimSpace(text))
	if m == nil {
		return "", 0, false
	}
	unit := strings.ToLower(m[1])
	amount, err := strconv.ParseFloat(m[2], 64)
	if err != nil {
		return "", 0, false
	}
	return unit, amount, true
}

// parseAccountCommand parses "/account cbe 1234567890"
func parseAccountCommand(text string) (string, bool) {
	re := regexp.MustCompile(`(?i)^/account\s+(.+)`)
	m := re.FindStringSubmatch(strings.TrimSpace(text))
	if m == nil {
		return "", false
	}
	return strings.TrimSpace(m[1]), true
}

// parseDeclineCommand parses "/decline reason"
func parseDeclineCommand(text string) (string, bool) {
	re := regexp.MustCompile(`(?i)^/decline\s*(.*)`)
	m := re.FindStringSubmatch(strings.TrimSpace(text))
	if m == nil {
		return "", false
	}
	return strings.TrimSpace(m[1]), true
}

// parsePaidCleaner parses "/paidcleaner 3000"
func parsePaidCleaner(text string) (float64, bool) {
	re := regexp.MustCompile(`(?i)^/paidcleaner\s+(\d+(?:\.\d+)?)`)
	m := re.FindStringSubmatch(strings.TrimSpace(text))
	if m == nil {
		return 0, false
	}
	amount, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, false
	}
	return amount, true
}

// parsePaidMaterials parses "/paidmaterials 200"
func parsePaidMaterials(text string) (float64, bool) {
	re := regexp.MustCompile(`(?i)^/paidmaterials\s+(\d+(?:\.\d+)?)`)
	m := re.FindStringSubmatch(strings.TrimSpace(text))
	if m == nil {
		return 0, false
	}
	amount, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, false
	}
	return amount, true
}

// allPaid returns true if all 16 units have paid
func allPaid(payments map[string]*db.TahorPayment) bool {
	return len(payments) == 16
}

// pendingUnits returns units that have not paid
func pendingUnits(payments map[string]*db.TahorPayment) []string {
	var pending []string
	for _, unit := range allUnits {
		if _, ok := payments[unit]; !ok {
			pending = append(pending, strings.ToUpper(unit))
		}
	}
	sort.Strings(pending)
	return pending
}

// buildPaymentSummary builds compact payment status
func buildPaymentSummary(payments map[string]*db.TahorPayment) string {
	var sb strings.Builder
	sb.WriteString("💰 *የጽዳት ፈንድ ክፍያ ሁኔታ*\n\n")

	var paidList, pendingList []string
	for _, unit := range allUnits {
		if _, ok := payments[unit]; ok {
			paidList = append(paidList, strings.ToUpper(unit))
		} else {
			pendingList = append(pendingList, strings.ToUpper(unit))
		}
	}

	if len(paidList) > 0 {
		sb.WriteString("✅ *ከፍለዋል:*\n")
		for i, u := range paidList {
			sb.WriteString(fmt.Sprintf("%-6s", u))
			if (i+1)%8 == 0 {
				sb.WriteString("\n")
			}
		}
		if len(paidList)%8 != 0 {
			sb.WriteString("\n")
		}
	}

	if len(pendingList) > 0 {
		sb.WriteString(fmt.Sprintf("\n❌ *ያልከፈሉ:* %s\n", strings.Join(pendingList, ", ")))
	}

	totalCollected := 0.0
	for _, p := range payments {
		totalCollected += p.Amount
	}
	sb.WriteString(fmt.Sprintf("\n*ከፍለዋል: %d/16 | ይቀራቸዋል: %d*", len(paidList), len(pendingList)))
	sb.WriteString(fmt.Sprintf("\n💰 *እስካሁን የተሰበሰበ: %.0f ብር*", totalCollected))
	return sb.String()
}

// buildBalance builds the balance summary deducting all expenses
func buildBalance(payments map[string]*db.TahorPayment, ledger []*db.TahorLedger) string {
	var totalCollected, totalExpenses float64

	for _, p := range payments {
		totalCollected += p.Amount
	}

	var sb strings.Builder
	sb.WriteString("📊 *የጽዳት ፈንድ ሂሳብ*\n\n")
	sb.WriteString(fmt.Sprintf("📥 የተሰበሰበ: *%.0f ብር*\n", totalCollected))

	if len(ledger) > 0 {
		sb.WriteString("\n📤 *ወጪዎች:*\n")
		for _, e := range ledger {
			sb.WriteString(fmt.Sprintf("  — %s: *%.0f ብር*\n", e.Type, e.Amount))
			totalExpenses += e.Amount
		}
	}

	balance := totalCollected - totalExpenses
	sb.WriteString(fmt.Sprintf("\n💰 *ቀሪ ሂሳብ: %.0f ብር*", balance))
	return sb.String()
}

// parseCleanedCommand parses "tahor cleaned 3"
func parseCleanedCommand(text string) (int, bool) {
	re := regexp.MustCompile(`(?i)^tahor\s+cleaned\s+(\d+)`)
	m := re.FindStringSubmatch(strings.TrimSpace(text))
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, false
	}
	return n, true
}

// parseResetCleaned parses "tahor reset cleaned 3"
func parseResetCleaned(text string) (int, bool) {
	re := regexp.MustCompile(`(?i)^tahor\s+reset\s+cleaned\s+(\d+)`)
	m := re.FindStringSubmatch(strings.TrimSpace(text))
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, false
	}
	return n, true
}

// parseTahorStart parses "tahor start"
func parseTahorStart(text string) bool {
	return regexp.MustCompile(`(?i)^tahor\s+start$`).MatchString(strings.TrimSpace(text))
}

// parseTahorEnd parses "tahor end"
func parseTahorEnd(text string) bool {
	return regexp.MustCompile(`(?i)^tahor\s+end$`).MatchString(strings.TrimSpace(text))
}

// parseTahorExpense parses "tahor expense 3000 reason here"
func parseTahorExpense(text string) (float64, string, bool) {
	re := regexp.MustCompile(`(?i)^tahor\s+expense\s+(\d+(?:\.\d+)?)\s+(.+)`)
	m := re.FindStringSubmatch(strings.TrimSpace(text))
	if m == nil {
		return 0, "", false
	}
	amount, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, "", false
	}
	return amount, strings.TrimSpace(m[2]), true
}

// buildCleaningSummary builds cleaning progress summary
func buildCleaningSummary(sessions []int, totalSessions int) string {
	nextExpected := len(sessions) + 1
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🧹 *የጽዳት ሁኔታ*\n\n"))
	sb.WriteString(fmt.Sprintf("✅ *የጸዳ:* %d/%d\n", len(sessions), totalSessions))
	sb.WriteString(fmt.Sprintf("⏳ *የቀረ:* %d\n", totalSessions-len(sessions)))
	if nextExpected <= totalSessions {
		sb.WriteString(fmt.Sprintf("\n📌 *የሚጠበቀው ክፍለ ጊዜ:* %d", nextExpected))
	} else {
		sb.WriteString("\n🎉 *ሁሉም ክፍለ ጊዜዎች ተጠናቅቀዋል!*")
	}
	return sb.String()
}

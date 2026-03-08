package tahor

import (
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/avvvet/siloam/config"
	"github.com/avvvet/siloam/db"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const footer = "\n\n— ታሆር"

const introMessage = `የጽዳት ጉዳይ ላይ አስተያየታችሁን እንድትሰጡ ተጠይቃችሁ ነበር፣ ምላሽ ባለመገኘቱ — እኔ ታሆር ለመፍትሄ ተሰይሜያለሁ!

👋 *ሰላም የአፓርታማ ቤተሰቦች!*
እኔ *ታሆር* ነኝ — ትርጉሜ *ንጹህ* ማለት ነው።
የአፓርታማችሁን የጽዳት አስተዳደር ለመርዳት እዚህ ተሰይሜያለሁ።

*እኔ የምሠራው ይህንን ነው:*
🧹 በየ 3 ወሩ ከእያንዳንዱ ቤት 600 ብር እንዲሰበሰብ አደርጋለሁ *(በወር 200 ብር ማለት ነው)*
👤 በ 3 ወር አንዴ አብሮ የሚረዳኝን ቤት በዕጣ መርጬ ለቡድኑ አሳውቃለሁ
💰 16ቱም ቤቶች እስኪከፍሉ ድረስ ክፍያን እከታተላለሁ
🧾 የጽዳት ሠራተኛ ወርሃዊ ክፍያ እከታተላለሁ
📢 ሁሉም ነገር እስኪጠናቀቅ ቡድኑን በወቅቱ አሳውቃለሁ

*📌 በዕጣ የሚመረጠው ቤት የሚያረዳኝ እንዲህ ነው:*
— የቴሌ ብር ወይም የባንክ አካውንት ቁጥር ለቡድኑ ያጋሩ
— የተሰበሰበውን ፈንድ ይቀበሉ
— ለጽዳት ሠራተኛ በወር 3,000 ብር ይክፈሉ
— የጽዳት ዕቃ ሲያልቅ ይግዙ
— ክፍያ ሲፈጸም እዚህ ግሩፕ ላይ ያረጋግጡ
_(ከቤተሰብ ውስጥ አንድ ሰው መወከል ይቻላል)_

⏰ *ነገ እሁድ ከቀኑ 9 ሰዓት ላይ የመጀመሪያውን ዕጣ አወጣለሁ!*
ዕጣ የወጣለት ቤት ወዲያው የክፍያ መሰብሰቢያ አካውንት ይላኩልኝ!`

type Bot struct {
	api *tgbotapi.BotAPI
	db  *db.DB
	cfg *config.Config
}

func New(cfg *config.Config, database *db.DB) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(cfg.TahorBotToken)
	if err != nil {
		return nil, err
	}
	log.Printf("Tahor authorized as @%s", api.Self.UserName)

	if err := database.InitTahorBuckets(); err != nil {
		return nil, err
	}

	return &Bot{api: api, db: database, cfg: cfg}, nil
}

func (b *Bot) Start() {
	b.startScheduler()
	b.StartPreDrawReminders() // send reminder immediately if draw not done yet

	// Uncomment to post intro once
	// b.sendToGroup(introMessage + footer)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := b.api.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}
		go b.handleMessage(update.Message)
	}
}

func (b *Bot) handleMessage(msg *tgbotapi.Message) {
	// DM
	if !msg.Chat.IsGroup() && !msg.Chat.IsSuperGroup() {
		b.sendDMReply(msg.Chat.ID)
		return
	}

	// Only handle configured group
	if msg.Chat.ID != b.cfg.TahorGroupID {
		return
	}

	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return
	}

	// /account
	if account, ok := parseAccountCommand(text); ok {
		b.handleAccount(msg, account)
		return
	}

	// /decline
	if reason, ok := parseDeclineCommand(text); ok {
		b.handleDecline(msg, reason)
		return
	}

	// /startcleaner
	if strings.EqualFold(text, "/startcleaner") {
		b.handleStartCleaner(msg)
		return
	}

	// /paidcleaner
	if amount, ok := parsePaidCleaner(text); ok {
		b.handlePaidCleaner(msg, amount)
		return
	}

	// /paidmaterials
	if amount, ok := parsePaidMaterials(text); ok {
		b.handlePaidMaterials(msg, amount)
		return
	}

	// /balance
	if strings.EqualFold(text, "/balance") {
		b.handleBalance(msg)
		return
	}

	// tahor a=600
	if unit, amount, ok := parseTahorPayment(text); ok {
		b.handleFundPayment(msg, unit, amount)
		return
	}
}

func (b *Bot) runDraw() {
	cycle, _ := b.db.GetTahorCycle()

	usedUnits := []string{}
	if cycle != nil {
		usedUnits = cycle.UsedUnits
	}

	// Reset if all 16 used
	if len(usedUnits) >= 16 {
		usedUnits = []string{}
	}

	// Pick random unit not in usedUnits
	available := []string{}
	for _, u := range allUnits {
		used := false
		for _, uu := range usedUnits {
			if uu == u {
				used = true
				break
			}
		}
		if !used {
			available = append(available, u)
		}
	}

	rand.Seed(time.Now().UnixNano())
	selected := available[rand.Intn(len(available))]

	// Save new cycle
	cycleID := fmt.Sprintf("%s-C%d", time.Now().Format("2006"), len(usedUnits)+1)
	newCycle := &db.TahorCycle{
		ID:        cycleID,
		StartedAt: time.Now(),
		Active:    true,
		UsedUnits: append(usedUnits, selected),
	}
	b.db.SaveTahorCycle(newCycle)

	// Save delegate
	delegate := &db.TahorDelegate{
		Unit:     selected,
		CycleID:  cycleID,
		Selected: time.Now(),
	}
	b.db.SaveTahorDelegate(delegate)

	b.sendToGroup(fmt.Sprintf(
		"🎉 *ዕጣ ወጣ!*\n\n*ቤት %s* ለሚቀጥሉት 3 ወራት አብሮ የሚረዳኝ ቤት ሆኖ ተመርጧል!\n\nእባክዎ ወድያው የክፍያ መሰብሰቢያ አካውንትዎን ያጋሩ:\n`/account cbe 1234567890`\n\n_(ከቤተሰብ ውስጥ አንድ ሰው መወከል ይቻላል)_%s",
		strings.ToUpper(selected), footer,
	))
}

func (b *Bot) handleAccount(msg *tgbotapi.Message, account string) {
	delegate, err := b.db.GetTahorDelegate()
	if err != nil || delegate == nil {
		return
	}

	delegate.Account = account
	b.db.SaveTahorDelegate(delegate)

	b.sendToGroup(fmt.Sprintf(
		"✅ *ረዳት ቤት %s አካውንት አጋርተዋል!*\n\n📌 *የክፍያ አካውንት:* _%s_\n\nእባክዎ 600 ብር ወደዚህ አካውንት ይላኩ! ከከፈሉ በኋላ እንዲህ ይጻፉ — ምሳሌ: `tahor a=600`\n\n💛 ፈቃደኛ ከሆኑ ለረዳት ቤት %s ምስጋና ምልክት ሆኖ 100 ብር ተጨምረው መላክ ይችላሉ — ምንም ግዴታ የለም! 🙏%s",
		strings.ToUpper(delegate.Unit), account, strings.ToUpper(delegate.Unit), footer,
	))
}

func (b *Bot) handleDecline(msg *tgbotapi.Message, reason string) {
	delegate, err := b.db.GetTahorDelegate()
	if err != nil || delegate == nil {
		return
	}

	// Only the selected delegate can decline
	// (we check by unit — in real use member would include their unit)
	delegate.Declined = true
	b.db.SaveTahorDelegate(delegate)

	b.sendToGroup(fmt.Sprintf(
		"ℹ️ *ቤት %s 거절 አድርጓል*\nምክንያት: %s\n\nሌላ ረዳት እየመረጥኩ ነው...%s",
		strings.ToUpper(delegate.Unit), reason, footer,
	))

	// Re-run draw
	b.runDraw()
}

func (b *Bot) handleStartCleaner(msg *tgbotapi.Message) {
	cycle, err := b.db.GetTahorCycle()
	if err != nil || cycle == nil {
		return
	}

	payments, _ := b.db.GetTahorPayments(cycle.ID)
	if !allPaid(payments) {
		b.replyMarkdown(msg, "⚠️ ገና 16ቱም ቤቶች አልከፈሉም። ሁሉም ከፍለው ሲጠናቀቅ ይሞክሩ።"+footer)
		return
	}

	cycle.CleanerActive = true
	b.db.SaveTahorCycle(cycle)

	b.sendToGroup(fmt.Sprintf(
		"🧹 *የጽዳት አገልግሎት ተጀምሯል!*\n\nረዳት ቤት %s ክፍያ ሲፈጽሙ:\n`/paidcleaner 3000` — ለጽዳት ሠራተኛ\n`/paidmaterials 200` — ለጽዳት ዕቃ\n\nሒሳብ ለማየት: `/balance`%s",
		strings.ToUpper(cycle.UsedUnits[len(cycle.UsedUnits)-1]), footer,
	))
}

func (b *Bot) handleFundPayment(msg *tgbotapi.Message, unit string, amount float64) {
	cycle, err := b.db.GetTahorCycle()
	if err != nil || cycle == nil || !cycle.Active {
		return
	}

	delegate, _ := b.db.GetTahorDelegate()
	if delegate == nil || delegate.Account == "" {
		return // wait for account first
	}

	if amount < fundAmount {
		b.replyMarkdown(msg, fmt.Sprintf("❌ ቤት *%s* የተከፈለው ብር (%.0f) ከሚጠበቀው (600 ብር) ያነሰ ነው።%s", strings.ToUpper(unit), amount, footer))
		return
	}

	b.db.SaveTahorPayment(cycle.ID, unit, amount)

	payments, _ := b.db.GetTahorPayments(cycle.ID)
	b.sendToGroup(buildPaymentSummary(payments) + footer)

	if allPaid(payments) {
		cycle.FundCollected = true
		b.db.SaveTahorCycle(cycle)
		b.sendToGroup(fmt.Sprintf(
			"🎊 *16ቱም ቤቶች ከፍለዋል!*\n\nጠቅላላ 9,600 ብር ተሰብስቧል።\n\nረዳት ቤት %s እባክዎ የጽዳት ሠራተኛ አስጀምሩ እና ሲጀምሩ:\n`/startcleaner` ይጻፉ%s",
			strings.ToUpper(delegate.Unit), footer,
		))
	}
}

func (b *Bot) handlePaidCleaner(msg *tgbotapi.Message, amount float64) {
	cycle, err := b.db.GetTahorCycle()
	if err != nil || cycle == nil || !cycle.CleanerActive {
		return
	}

	b.db.AddTahorLedgerEntry(cycle.ID, "cleaner", amount)

	payments, _ := b.db.GetTahorPayments(cycle.ID)
	ledger, _ := b.db.GetTahorLedger(cycle.ID)

	b.sendToGroup(fmt.Sprintf("✅ *ለጽዳት ሠራተኛ %.0f ብር ተከፍሏል!*\n\n%s%s", amount, buildBalance(payments, ledger), footer))
}

func (b *Bot) handlePaidMaterials(msg *tgbotapi.Message, amount float64) {
	cycle, err := b.db.GetTahorCycle()
	if err != nil || cycle == nil || !cycle.CleanerActive {
		return
	}

	b.db.AddTahorLedgerEntry(cycle.ID, "materials", amount)

	payments, _ := b.db.GetTahorPayments(cycle.ID)
	ledger, _ := b.db.GetTahorLedger(cycle.ID)

	b.sendToGroup(fmt.Sprintf("✅ *ለጽዳት ዕቃ %.0f ብር ተከፍሏል!*\n\n%s%s", amount, buildBalance(payments, ledger), footer))
}

func (b *Bot) handleBalance(msg *tgbotapi.Message) {
	cycle, err := b.db.GetTahorCycle()
	if err != nil || cycle == nil {
		b.replyMarkdown(msg, "ℹ️ እስካሁን ምንም ዑደት አልተጀመረም።"+footer)
		return
	}

	payments, _ := b.db.GetTahorPayments(cycle.ID)
	ledger, _ := b.db.GetTahorLedger(cycle.ID)

	b.replyMarkdown(msg, buildBalance(payments, ledger)+footer)
}

func (b *Bot) sendDMReply(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "🤖 እኔ በቡድኑ ውስጥ ብቻ እሰራለሁ።"+footer)
	b.api.Send(msg)
}

func (b *Bot) replyMarkdown(orig *tgbotapi.Message, text string) {
	msg := tgbotapi.NewMessage(orig.Chat.ID, text)
	msg.ParseMode = "Markdown"
	msg.ReplyToMessageID = orig.MessageID
	b.api.Send(msg)
}

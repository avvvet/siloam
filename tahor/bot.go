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

const footer = "\n\n— Tahor | Cleaning Fund Manager"

const totalCleaningSessions = 16

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

	// /balance
	if strings.EqualFold(text, "/balance") {
		b.handleBalance(msg)
		return
	}

	// tahor start
	if parseTahorStart(text) {
		b.handleTahorStart(msg)
		return
	}

	// tahor end
	if parseTahorEnd(text) {
		b.handleTahorEnd(msg)
		return
	}

	// tahor cleaned 3
	if session, ok := parseCleanedCommand(text); ok {
		b.handleCleaned(msg, session)
		return
	}

	// tahor reset cleaned 3
	if session, ok := parseResetCleaned(text); ok {
		b.handleResetCleaned(msg, session)
		return
	}

	// tahor expense 3000 reason
	if amount, reason, ok := parseTahorExpense(text); ok {
		b.handleExpense(msg, amount, reason)
		return
	}

	// tahor reset a
	if unit, ok := parseTahorReset(text); ok {
		b.handleResetPayment(msg, unit)
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

	if len(usedUnits) >= 16 {
		usedUnits = []string{}
	}

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

	cycleID := fmt.Sprintf("%s-C%d", time.Now().Format("2006"), len(usedUnits)+1)
	newCycle := &db.TahorCycle{
		ID:        cycleID,
		StartedAt: time.Now(),
		Active:    true,
		UsedUnits: append(usedUnits, selected),
	}
	b.db.SaveTahorCycle(newCycle)

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

	delegate.Declined = true
	b.db.SaveTahorDelegate(delegate)

	b.sendToGroup(fmt.Sprintf(
		"ℹ️ *ቤት %s 거절 አድርጓል*\nምክንያት: %s\n\nሌላ ረዳት እየመረጥኩ ነው...%s",
		strings.ToUpper(delegate.Unit), reason, footer,
	))

	b.runDraw()
}

func (b *Bot) handleTahorStart(msg *tgbotapi.Message) {
	cycle, err := b.db.GetTahorCycle()
	if err != nil || cycle == nil {
		b.replyMarkdown(msg, "⚠️ ምንም ዑደት አልተጀመረም።"+footer)
		return
	}

	cycle.CleanerActive = true
	b.db.SaveTahorCycle(cycle)

	b.sendToGroup(
		"🧹 *የጽዳት አገልግሎት ተጀምሯል!*\n\n" +
			"📅 ጽዳት: ረቡዕ እና ቅዳሜ — በወር 8 ጊዜ\n" +
			"📊 ጠቅላላ: 16 ክፍለ ጊዜ\n\n" +
			"ጽዳቱ ሲጠናቀቅ: tahor cleaned 1 ይጻፉ" + footer)
}

func (b *Bot) handleTahorEnd(msg *tgbotapi.Message) {
	cycle, err := b.db.GetTahorCycle()
	if err != nil || cycle == nil {
		return
	}

	sessions, _ := b.db.GetCleaningSessions(cycle.ID)
	payments, _ := b.db.GetTahorPayments(cycle.ID)
	ledger, _ := b.db.GetTahorLedger(cycle.ID)

	b.sendToGroup(fmt.Sprintf(
		"✅ *የጽዳት አገልግሎት ተጠናቋል!*\n\n"+
			"🧹 *የጸዳ ክፍለ ጊዜ:* %d/%d\n\n"+
			"%s\n\n"+
			"እናመሰግናለን! 🙏%s",
		len(sessions), totalCleaningSessions, buildBalance(payments, ledger), footer))
}

func (b *Bot) handleCleaned(msg *tgbotapi.Message, session int) {
	cycle, err := b.db.GetTahorCycle()
	if err != nil || cycle == nil || !cycle.CleanerActive {
		return
	}

	if session > totalCleaningSessions {
		b.replyMarkdown(msg, fmt.Sprintf("⚠️ ክፍለ ጊዜ %d የለም። ጠቅላላ ክፍለ ጊዜ %d ነው።%s", session, totalCleaningSessions, footer))
		return
	}

	if b.db.IsSessionConfirmed(cycle.ID, session) {
		b.replyMarkdown(msg, fmt.Sprintf("ℹ️ ክፍለ ጊዜ %d አስቀድሞ ተረጋግጧል!%s", session, footer))
		return
	}

	sessions, _ := b.db.GetCleaningSessions(cycle.ID)
	nextExpected := len(sessions) + 1

	if session != nextExpected {
		b.replyMarkdown(msg, fmt.Sprintf("⚠️ አሁን የሚጠበቀው ክፍለ ጊዜ %d ነው። `tahor cleaned %d` ይጻፉ%s", nextExpected, nextExpected, footer))
		return
	}

	b.db.SaveCleaningSession(cycle.ID, session)
	sessions, _ = b.db.GetCleaningSessions(cycle.ID)

	if session == totalCleaningSessions {
		b.sendToGroup(fmt.Sprintf("🎉 *ክፍለ ጊዜ %d/%d ተረጋግጧል!*\n\nሁሉም ክፍለ ጊዜዎች ተጠናቅቀዋል! `tahor end` ይጻፉ%s", session, totalCleaningSessions, footer))
	} else {
		next := session + 1
		b.sendToGroup(fmt.Sprintf("✅ *ክፍለ ጊዜ %d/%d ተረጋግጧል!*\n\nክፍለ ጊዜ %d ሲጠናቀቅ: `tahor cleaned %d` ይጻፉ%s", session, totalCleaningSessions, next, next, footer))
	}
}

func (b *Bot) handleResetCleaned(msg *tgbotapi.Message, session int) {
	cycle, err := b.db.GetTahorCycle()
	if err != nil || cycle == nil {
		return
	}

	if !b.db.IsSessionConfirmed(cycle.ID, session) {
		b.replyMarkdown(msg, fmt.Sprintf("ℹ️ ክፍለ ጊዜ %d ተረጋግጦ አልነበረም።%s", session, footer))
		return
	}

	b.db.DeleteCleaningSession(cycle.ID, session)
	sessions, _ := b.db.GetCleaningSessions(cycle.ID)
	nextExpected := len(sessions) + 1
	b.sendToGroup(fmt.Sprintf("🔄 *ክፍለ ጊዜ %d ተሰርዟል።* አሁን የሚጠበቀው ክፍለ ጊዜ %d ነው።%s", session, nextExpected, footer))
}

func (b *Bot) handleExpense(msg *tgbotapi.Message, amount float64, reason string) {
	cycle, err := b.db.GetTahorCycle()
	if err != nil || cycle == nil {
		return
	}

	b.db.AddTahorLedgerEntry(cycle.ID, reason, amount)

	payments, _ := b.db.GetTahorPayments(cycle.ID)
	ledger, _ := b.db.GetTahorLedger(cycle.ID)
	b.sendToGroup(fmt.Sprintf("📤 *%.0f ብር (%s) ወጪ ተመዝግቧል!*\n\n%s%s", amount, reason, buildBalance(payments, ledger), footer))
}

func (b *Bot) handleFundPayment(msg *tgbotapi.Message, unit string, amount float64) {
	cycle, err := b.db.GetTahorCycle()
	if err != nil || cycle == nil || !cycle.Active {
		return
	}

	delegate, _ := b.db.GetTahorDelegate()
	if delegate == nil || delegate.Account == "" {
		return
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
			"🎊 *16ቱም ቤቶች ከፍለዋል!*\n\nጠቅላላ 9,600 ብር ተሰብስቧል።\n\nረዳት ቤት %s እባክዎ የጽዳት ሠራተኛ ሲጀምሩ:\n`tahor start` ይጻፉ%s",
			strings.ToUpper(delegate.Unit), footer,
		))
	}
}

func (b *Bot) handleBalance(msg *tgbotapi.Message) {
	cycle, err := b.db.GetTahorCycle()
	if err != nil || cycle == nil {
		b.replyMarkdown(msg, "ℹ️ እስካሁን ምንም ዑደት አልተጀመረም።"+footer)
		return
	}

	payments, _ := b.db.GetTahorPayments(cycle.ID)
	ledger, _ := b.db.GetTahorLedger(cycle.ID)
	sessions, _ := b.db.GetCleaningSessions(cycle.ID)

	text := buildBalance(payments, ledger)
	if cycle.CleanerActive {
		text += fmt.Sprintf("\n\n🧹 *የጸዳ ክፍለ ጊዜ:* %d/%d", len(sessions), totalCleaningSessions)
	}
	b.replyMarkdown(msg, text+footer)
}

func (b *Bot) handleResetPayment(msg *tgbotapi.Message, unit string) {
	cycle, err := b.db.GetTahorCycle()
	if err != nil || cycle == nil || !cycle.Active {
		return
	}

	if err := b.db.DeleteTahorPayment(cycle.ID, unit); err != nil {
		b.replyMarkdown(msg, "❌ ክፍያ ማስወገድ አልተቻለም።"+footer)
		return
	}

	payments, _ := b.db.GetTahorPayments(cycle.ID)
	b.sendToGroup(fmt.Sprintf("🔄 *ቤት %s* ክፍያ ተሰርዟል።\n\n%s%s", strings.ToUpper(unit), buildPaymentSummary(payments), footer))
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

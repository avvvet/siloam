package bot

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/avvvet/siloam/config"
	"github.com/avvvet/siloam/db"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const footer = "\n\n— 🤖 Hi, my name is Siloam | Not a human"

type Bot struct {
	api *tgbotapi.BotAPI
	db  *db.DB
	cfg *config.Config
}

func New(cfg *config.Config, database *db.DB) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(cfg.BotToken)
	if err != nil {
		return nil, err
	}
	log.Printf("Authorized as @%s", api.Self.UserName)
	return &Bot{api: api, db: database, cfg: cfg}, nil
}

func (b *Bot) Start() {
	b.startScheduler()

	// Uncomment to greet on every startup
	// b.sendIntro(b.cfg.GroupID)

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
	// Bot added to a group — uncomment to greet on join
	// if msg.NewChatMembers != nil {
	// 	for _, member := range msg.NewChatMembers {
	// 		if member.ID == b.api.Self.ID {
	// 			b.sendIntro(msg.Chat.ID)
	// 			return
	// 		}
	// 	}
	// }

	// DM — not a group
	if !msg.Chat.IsGroup() && !msg.Chat.IsSuperGroup() {
		b.sendDMReply(msg.Chat.ID)
		return
	}

	// Only handle messages in the configured group
	if msg.Chat.ID != b.cfg.GroupID {
		return
	}

	// Respond when bot is mentioned
	if msg.Entities != nil {
		for _, e := range msg.Entities {
			if e.Type == "mention" {
				mention := msg.Text[e.Offset : e.Offset+e.Length]
				if mention == "@"+b.api.Self.UserName {
					b.sendStatus(msg)
					return
				}
			}
		}
	}

	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return
	}

	// Handle /bill command
	if amount, ok := parseBillCommand(text); ok {
		b.handleBillCommand(msg, amount)
		return
	}

	// Handle payment submission
	if payments := parsePayments(text); len(payments) > 0 {
		b.handlePayments(msg, payments)
		return
	}

	// Handle reading submission
	if parsed := parseReadings(text); len(parsed) > 0 {
		b.handleReadings(msg, parsed)
		return
	}
}

func (b *Bot) handleReadings(msg *tgbotapi.Message, parsed map[string]int) {
	if !isSubmissionOpen() {
		var reply string
		if isBeforeWindow() {
			reply = fmt.Sprintf("⏳ Readings are not open yet. Next submission date is *%s*.", nextSixth())
		} else {
			reply = fmt.Sprintf("❌ Submission period has closed. Next reading date is *%s*.", nextSixth())
		}
		b.replyMarkdown(msg, reply+footer)
		return
	}

	var confirmed []string
	for unit, value := range parsed {
		reading, err := b.db.SaveReading(unit, value)
		if err != nil {
			log.Printf("Error saving reading for unit %s: %v", unit, err)
			continue
		}
		if reading.Updated {
			confirmed = append(confirmed, fmt.Sprintf("🔄 *%s:* %d _(was %d)_", strings.ToUpper(unit), value, reading.OldValue))
		} else {
			confirmed = append(confirmed, fmt.Sprintf("✅ *%s:* %d", strings.ToUpper(unit), value))
		}
	}

	if len(confirmed) == 0 {
		return
	}

	b.replyMarkdown(msg, fmt.Sprintf("*Recorded:*\n%s%s", strings.Join(confirmed, "\n"), footer))

	readings, err := b.db.GetAllReadings()
	if err != nil {
		return
	}
	b.sendToGroup(buildSummary(readings) + footer)

	// All 16 submitted — prompt for bill
	if allSubmitted(readings) {
		b.sendToGroup(fmt.Sprintf("🎉 *All readings submitted!*\nPost the total bill amount using:\n`/bill 5000`%s", footer))
	}
}

func (b *Bot) handleBillCommand(msg *tgbotapi.Message, amount float64) {
	// Check all readings submitted
	readings, err := b.db.GetAllReadings()
	if err != nil || !allSubmitted(readings) {
		b.replyMarkdown(msg, "⚠️ Cannot post bill yet — not all 16 readings submitted."+footer)
		return
	}

	// Save bill (not finalized yet, finalized at midnight)
	bill := &db.Bill{
		TotalBill: amount,
		Units:     make(map[string]float64),
		Diffs:     make(map[string]int),
		Percents:  make(map[string]float64),
		Previous:  make(map[string]int),
		Current:   make(map[string]int),
	}
	if err := b.db.SaveBill(bill); err != nil {
		log.Printf("Error saving bill: %v", err)
		return
	}

	b.replyMarkdown(msg, fmt.Sprintf(
		"✅ *Bill amount set: %.0f Birr*\nYou can update it with `/bill [amount]` until midnight.\nFinal calculation will be posted at midnight.%s",
		amount, footer,
	))
}

func (b *Bot) handlePayments(msg *tgbotapi.Message, payments map[string]float64) {
	bill, err := b.db.GetBill()
	if err != nil || bill == nil || !bill.Finalized {
		return // silently ignore if bill not finalized
	}

	var confirmed []string
	for unit, amount := range payments {
		owed := bill.Units[unit]
		if amount < owed {
			confirmed = append(confirmed, fmt.Sprintf("❌ *%s:* %.0f Birr is less than owed %.0f Birr", strings.ToUpper(unit), amount, owed))
			continue
		}
		payment, err := b.db.SavePayment(unit, amount)
		if err != nil {
			log.Printf("Error saving payment for unit %s: %v", unit, err)
			continue
		}
		if payment.Updated {
			confirmed = append(confirmed, fmt.Sprintf("🔄 *%s:* %.0f Birr _(updated from %.0f)_", strings.ToUpper(unit), amount, payment.OldAmount))
		} else {
			confirmed = append(confirmed, fmt.Sprintf("✅ *%s:* %.0f Birr paid", strings.ToUpper(unit), amount))
		}
	}

	if len(confirmed) > 0 {
		b.replyMarkdown(msg, fmt.Sprintf("*Payment Recorded:*\n%s%s", strings.Join(confirmed, "\n"), footer))
	}

	allPayments, err := b.db.GetAllPayments()
	if err != nil {
		return
	}

	b.sendToGroup(buildPaymentSummary(allPayments, bill.Units) + footer)

	// All paid
	if allPaid(allPayments, bill.Units) {
		b.sendToGroup(fmt.Sprintf(
			"🎊 *All payments received!*\n\nThis month's water bill has ended.\nPlease pay *%.0f Birr* to the water authority today.\n\nSee you next month! 💧%s",
			bill.TotalBill, footer,
		))
	}
}

func (b *Bot) sendStatus(msg *tgbotapi.Message) {
	now := time.Now().UTC()
	text := fmt.Sprintf(
		"🟢 I'm alive and running!\n🕐 Server time: %s\n📅 Next submission date: %s",
		now.Format("Mon, 02 Jan 2006 15:04:05 UTC"),
		nextSixth(),
	)
	b.replyMarkdown(msg, text+footer)
}

func (b *Bot) sendIntro(chatID int64) {
	text := fmt.Sprintf(
		"👋 Hi! I'm *Siloam*, created by *%s*.\n\n"+
			"I'm here to manage the apartment water meter readings.\n\n"+
			"Every month on the *6th*, I'll remind everyone to submit their readings and keep track of all 16 units.\n\n"+
			"📌 Use this format to submit:\n`a=340, b=590, c=120`\n\n"+
			"One person can submit for multiple units at once.%s",
		b.cfg.BotCreator,
		footer,
	)
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	b.api.Send(msg)
}

func (b *Bot) sendDMReply(chatID int64) {
	text := fmt.Sprintf("🤖 I only work in the apartment group. Please submit your readings there.\n\nYou can contact my creator: %s", b.cfg.BotCreatorUsername) + footer
	msg := tgbotapi.NewMessage(chatID, text)
	b.api.Send(msg)
}

func (b *Bot) replyMarkdown(orig *tgbotapi.Message, text string) {
	msg := tgbotapi.NewMessage(orig.Chat.ID, text)
	msg.ParseMode = "Markdown"
	msg.ReplyToMessageID = orig.MessageID
	b.api.Send(msg)
}

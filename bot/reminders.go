package bot

import (
	"fmt"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/robfig/cron/v3"
)

func (b *Bot) startScheduler() {
	loc, _ := time.LoadLocation("Europe/Moscow")
	c := cron.New(cron.WithLocation(loc))

	// 5th at 8PM — eve reminder
	c.AddFunc("0 20 5 * *", func() {
		b.sendToGroup("📅 *Reminder:* Tomorrow is water reading day!")
	})

	// 6th at 8AM — morning reminder
	c.AddFunc("0 8 6 * *", func() {
		b.sendToGroup("📊 *Water Reading Day!*\nPlease submit your readings now.\nFormat: `a=340, b=590, c=120`")
	})

	// 6th at 1PM — midday reminder (only if pending)
	c.AddFunc("0 13 6 * *", func() {
		readings, err := b.db.GetAllReadings()
		if err != nil || len(readings) == 16 {
			return
		}
		pending := pendingUnits(readings)
		b.sendToGroup(fmt.Sprintf("⏳ *Midday reminder!*\nStill waiting for: *%s*", strings.Join(pending, ", ")))
	})

	// 6th at 8PM — evening warning (only if pending)
	c.AddFunc("0 20 6 * *", func() {
		readings, err := b.db.GetAllReadings()
		if err != nil || len(readings) == 16 {
			return
		}
		pending := pendingUnits(readings)
		b.sendToGroup(fmt.Sprintf("⚠️ *WARNING:* Submission closes at midnight!\nStill waiting for: *%s*", strings.Join(pending, ", ")))
	})

	// Midnight on 7th — finalize bill if posted
	c.AddFunc("0 0 7 * *", func() {
		b.finalizeBill()
	})

	// Every 4 hours — payment reminder
	c.AddFunc("0 */4 * * *", func() {
		b.sendPaymentReminder()
	})

	c.Start()
}

func (b *Bot) finalizeBill() {
	bill, err := b.db.GetBill()
	if err != nil || bill == nil || bill.Finalized {
		return
	}

	readings, err := b.db.GetAllReadings()
	if err != nil {
		return
	}

	// Get previous readings
	previous, err := b.db.GetPreviousReadings()
	if err != nil || len(previous) == 0 {
		previous = b.cfg.PreviousReadings
	}

	// Calculate bill
	computed := calculateBill(readings, previous, bill.TotalBill, b.cfg.AdditionalFee)
	computed.Finalized = true

	if err := b.db.SaveBill(computed); err != nil {
		return
	}

	// Generate and send PNG
	imgPath, err := generateBillImage(computed, b.cfg.BotCreatorUsername)
	if err != nil {
		b.sendToGroup("❌ Error generating bill image.")
		return
	}

	photo := tgbotapi.NewPhoto(b.cfg.GroupID, tgbotapi.FilePath(imgPath))
	photo.Caption = fmt.Sprintf("💧 *Water Bill generated*\nPlease pay within 3 days.\n\nAfter you pay please post exactly this format:\n_Example if your payment is 300 Birr, post:_\n`a=300birr`%s", footer)
	photo.ParseMode = "Markdown"
	b.api.Send(photo)
}

func (b *Bot) sendPaymentReminder() {
	bill, err := b.db.GetBill()
	if err != nil || bill == nil || !bill.Finalized {
		return
	}

	payments, err := b.db.GetAllPayments()
	if err != nil {
		return
	}

	pending := pendingPaymentUnits(payments, bill.Units)
	if len(pending) == 0 {
		return
	}

	b.sendToGroup(fmt.Sprintf("💳 *Payment Reminder!*\nStill waiting for: *%s*\n\nFormat: `a=300birr`%s",
		strings.Join(pending, ", "), footer))
}

func (b *Bot) sendToGroup(text string) {
	msg := tgbotapi.NewMessage(b.cfg.GroupID, text)
	msg.ParseMode = "Markdown"
	b.api.Send(msg)
}

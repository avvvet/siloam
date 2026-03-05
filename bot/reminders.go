package bot

import (
	"fmt"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/robfig/cron/v3"
)

func (b *Bot) startScheduler() {
	loc, _ := time.LoadLocation("Europe/Moscow") // GMT+3
	c := cron.New(cron.WithLocation(loc))

	// 5th of every month at 8:00 PM — eve reminder
	c.AddFunc("0 20 5 * *", func() {
		b.sendToGroup("📅 *Reminder:* Tomorrow is water reading day!")
	})

	// 6th at 8:00 AM — morning reminder
	c.AddFunc("0 8 6 * *", func() {
		b.sendToGroup("📊 *Water Reading Day!*\nPlease submit your readings now.\nFormat: `a=340, b=590, c=120`")
	})

	// 6th at 1:00 PM — midday reminder (only if pending)
	c.AddFunc("0 13 6 * *", func() {
		readings, err := b.db.GetAllReadings()
		if err != nil || len(readings) == 16 {
			return
		}
		pending := pendingUnits(readings)
		msg := fmt.Sprintf("⏳ *Midday reminder!*\nStill waiting for: *%s*", strings.Join(pending, ", "))
		b.sendToGroup(msg)
	})

	// 6th at 8:00 PM — evening warning (only if pending)
	c.AddFunc("0 20 6 * *", func() {
		readings, err := b.db.GetAllReadings()
		if err != nil || len(readings) == 16 {
			return
		}
		pending := pendingUnits(readings)
		msg := fmt.Sprintf("⚠️ *WARNING:* Submission closes at midnight!\nStill waiting for: *%s*", strings.Join(pending, ", "))
		b.sendToGroup(msg)
	})

	c.Start()
}

func (b *Bot) sendToGroup(text string) {
	msg := tgbotapi.NewMessage(b.cfg.GroupID, text)
	msg.ParseMode = "Markdown"
	b.api.Send(msg)
}

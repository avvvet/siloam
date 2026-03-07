package tahor

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/robfig/cron/v3"
)

var preDrawReminders = []string{
	`📢 *ትኩረት!* ነገ እሁድ ከቀኑ 9 ሰዓት ላይ *አብሮ የጽዳት አስተዳደሩን የሚረዳኝ ቤት ዕጣ ይወጣል!*

🧹 የጋራ መወጣጫ እና የመግቢያ ቦታ ጽዳት ለሁላችንም ጤና እና ምቾት አስፈላጊ ነው።
💰 በወር 200 ብር ብቻ — ሁሉም ቤት ለጋራ ጽዳት አስተዋጽኦ ያደርጋል።
👤 ዕጣ የወጣለት ቤት አካውንት ቁጥር ብቻ ያጋራል — እኔ ታሆር በዋናነት ሥራውን አግዛለሁ!
ለሁላችን ንጹህ እና ምቹ አፓርታማ! 🏠`,

	`📢 *ትኩረት!* ነገ እሁድ ከቀኑ 9 ሰዓት ላይ *አብሮ የጽዳት አስተዳደሩን የሚረዳኝ ቤት ዕጣ ይወጣል!*

🏠 ንጹህ አፓርታማ የሁላችንም ኩራት ነው።
💰 በወር 200 ብር — ትንሽ መዋጮ፣ ትልቅ ለውጥ!
👤 ዕጣ የወጣለት ቤት አካውንት ብቻ ያጋራል — እኔ ታሆር ቀሪውን እሠራለሁ!
አብረን እናድርገው! 💪`,

	`📢 *ትኩረት!* ነገ እሁድ ከቀኑ 9 ሰዓት ላይ *አብሮ የጽዳት አስተዳደሩን የሚረዳኝ ቤት ዕጣ ይወጣል!*

✨ ንጹህ ደረጃ እና መግቢያ — ለእያንዳንዱ ቤተሰብ የሚሰጥ ስጦታ ነው።
💰 16 ቤቶች አንድ ላይ ሲሆኑ 200 ብር ብቻ በቂ ነው!
👤 የተመረጠው ቤት አካውንት ይሰጣል — እኔ ታሆር ክፍያውን እከታተላለሁ!
ለንጹህ ቤታችን አንድ እንሁን! 🤝`,

	`📢 *ትኩረት!* ነገ እሁድ ከቀኑ 9 ሰዓት ላይ *አብሮ የጽዳት አስተዳደሩን የሚረዳኝ ቤት ዕጣ ይወጣል!*

🧹 ጽዱ ደረጃ — ደስተኛ ጎረቤቶች!
💰 200 ብር በወር — ቡና ዋጋ ነው፣ ግን ለሁላችን ለውጥ ያመጣል!
👤 ዕጣ የወጣለት ቤት አካውንት ቁጥር ብቻ ያጋራል — እኔ ታሆር ሁሉንም አስተዳድራለሁ!
አብረን ንጹህ አፓርታማ እንፍጠር! 🌟`,
}

func (b *Bot) startScheduler() {
	loc, _ := time.LoadLocation("Europe/Moscow")
	c := cron.New(cron.WithLocation(loc))

	// Every 2 hours on March 7 — pre-draw reminder
	c.AddFunc("0 */2 7 3 *", func() {
		b.sendPreDrawReminder()
	})

	// First draw — March 8 at 3PM GMT+3
	c.AddFunc("0 15 8 3 *", func() {
		b.runDraw()
	})

	// Every 1 hour — remind delegate to submit account if not done
	c.AddFunc("0 * * * *", func() {
		b.remindDelegateAccount()
	})

	// Every 4 hours — remind units to pay fund if not all paid
	c.AddFunc("0 */4 * * *", func() {
		b.remindFundPayment()
	})

	c.Start()
}

func (b *Bot) sendPreDrawReminder() {
	// Only send if draw has not happened yet
	cycle, _ := b.db.GetTahorCycle()
	if cycle != nil && cycle.Active {
		return
	}
	rand.Seed(time.Now().UnixNano())
	msg := preDrawReminders[rand.Intn(len(preDrawReminders))]
	b.sendToGroup(msg + footer)
}

func (b *Bot) remindDelegateAccount() {
	delegate, err := b.db.GetTahorDelegate()
	if err != nil || delegate == nil || delegate.Account != "" || delegate.Declined {
		return
	}

	b.sendToGroup(fmt.Sprintf(
		"⏰ *ቤት %s* እባክዎ የክፍያ አካውንትዎን ገና አልላኩም።\nይህንን ይጻፉ: `/account cbe 1234567890`%s",
		strings.ToUpper(delegate.Unit), footer,
	))
}

func (b *Bot) remindFundPayment() {
	cycle, err := b.db.GetTahorCycle()
	if err != nil || cycle == nil || !cycle.Active {
		return
	}

	delegate, err := b.db.GetTahorDelegate()
	if err != nil || delegate == nil || delegate.Account == "" {
		return
	}

	payments, err := b.db.GetTahorPayments(cycle.ID)
	if err != nil || allPaid(payments) {
		return
	}

	pending := pendingUnits(payments)
	b.sendToGroup(fmt.Sprintf(
		"💰 *የጽዳት ፈንድ አስታዋሽ!*\n\nእባክዎ 600 ብር ወደ ቤት %s አካውንት ይላኩ:\n_%s_\n\nያልከፈሉ: *%s*\n\nለማረጋገጥ: `tahor a=600`%s",
		strings.ToUpper(delegate.Unit),
		delegate.Account,
		strings.Join(pending, ", "),
		footer,
	))
}

func (b *Bot) sendToGroup(text string) {
	msg := tgbotapi.NewMessage(b.cfg.TahorGroupID, text)
	msg.ParseMode = "Markdown"
	b.api.Send(msg)
}

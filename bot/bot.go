package bot

import (
	"fmt"
	"log"
	"strings"

	"github.com/avvvet/siloam/config"
	"github.com/avvvet/siloam/db"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const footer = "\n\n— 👋 Hi ሰላም, my name is Siloam | Not a human ሰው አይደለሁም"

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
	b.sendIntro(b.cfg.GroupID) // greet on every startup

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
	// Bot added to a group
	if msg.NewChatMembers != nil {
		for _, member := range msg.NewChatMembers {
			if member.ID == b.api.Self.ID {
				b.sendIntro(msg.Chat.ID)
				return
			}
		}
	}

	// DM — not a group
	if !msg.Chat.IsGroup() && !msg.Chat.IsSuperGroup() {
		b.sendDMReply(msg.Chat.ID)
		return
	}

	// Only handle messages in the configured group
	if msg.Chat.ID != b.cfg.GroupID {
		return
	}

	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return
	}

	// Try to parse readings
	parsed := parseReadings(text)
	if len(parsed) == 0 {
		return // Not a reading message, ignore
	}

	// Check submission window
	if !isSubmissionOpen() {
		var reply string
		if isBeforeWindow() {
			reply = fmt.Sprintf("አልተጀመረም ገና! Readings are not open yet. Next submission date is *%s*.", nextSixth())
		} else {
			reply = fmt.Sprintf("የወሃ ንባብ ተዘግትዋል ❌ Submission period has closed. Next reading date is *%s*.", nextSixth())
		}
		b.replyMarkdown(msg, reply+footer)
		return
	}

	// Save each reading
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

	// Confirmation message
	confirmMsg := fmt.Sprintf("ተቀብያለሁ Recorded:*\n%s%s", strings.Join(confirmed, "\n"), footer)
	b.replyMarkdown(msg, confirmMsg)

	// Post updated summary to group
	readings, err := b.db.GetAllReadings()
	if err != nil {
		log.Printf("Error getting readings: %v", err)
		return
	}
	b.sendToGroup(buildSummary(readings) + footer)
}

func (b *Bot) sendIntro(chatID int64) {
	text := fmt.Sprintf(
		"👋 Hi ሰላም! I'm *Siloam ሲሎም እባላለሁ*, created by *%s*.\n\n"+
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

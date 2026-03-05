# Siloam 💧

A Telegram bot that manages monthly water meter readings for a 16-unit apartment group.

---

## Features

- 📅 Automated reminders on the 5th (eve) and 6th (morning, midday, evening)
- 📊 Accepts readings from any group member for one or multiple units
- 🔄 Overwrites readings within the submission window (6th only)
- 📋 Posts an updated summary after every submission
- ⚠️ Rejects submissions outside the submission window with a helpful message
- 🤖 Responds to DMs directing users to the group

---

## Project Structure

```
siloam/
├── main.go
├── go.mod
├── .env
├── bot/
│   ├── bot.go          # Bot setup, message handler, intro & DM reply
│   ├── readings.go     # Parsing, validation, summary builder
│   └── reminders.go    # Cron scheduler & reminder messages
├── config/
│   └── config.go       # Loads environment variables
└── db/
    └── db.go           # BoltDB storage (readings per month)
```

---

## Setup

### 1. Create a Telegram Bot
- Open Telegram and chat with [@BotFather](https://t.me/BotFather)
- Run `/newbot` and follow the steps
- Copy the **bot token**
- Run `/setprivacy` → select your bot → set to **Disable** so the bot can read all group messages

### 2. Get Your Group Chat ID
- Add [@userinfobot](https://t.me/userinfobot) to your group temporarily
- It will print the group's chat ID (a negative number e.g. `-1001234567890`)
- Remove it after

### 3. Configure `.env`

```env
BOT_TOKEN=your_telegram_bot_token_here
GROUP_ID=-1001234567890
BOT_CREATOR=Your Name Here
```

### 4. Run

```bash
go mod tidy
go run main.go
```

---

## Usage

### Submitting Readings
Any group member can submit one or more readings on the **6th of each month**:

```
a=340, b=590, c=120
```

- Case-insensitive (`A=340` works too)
- One message can include multiple units
- One member can submit on behalf of others

### Submission Window
| Time | Behavior |
|------|----------|
| Before the 6th | Bot replies: submission not open yet |
| On the 6th (00:00–23:59 GMT+3) | ✅ Accepted |
| After the 6th | Bot replies: submission closed |

### Auto Summary
After every submission the bot posts the full status of all 16 units (A–P) in the group.

---

## Reminder Schedule (GMT+3)

| When | Message |
|------|---------|
| 5th — 8:00 PM | 📅 Reminder: Tomorrow is water reading day! |
| 6th — 8:00 AM | 📊 Water Reading Day! Submit your readings |
| 6th — 1:00 PM | ⏳ Midday reminder with pending units (if any) |
| 6th — 8:00 PM | ⚠️ WARNING: Submission closes at midnight! |

Midday and evening reminders are skipped if all 16 units have already submitted.

---

## Tech Stack

| | |
|---|---|
| Language | Go |
| Telegram API | [go-telegram-bot-api](https://github.com/go-telegram-bot-api/telegram-bot-api) |
| Scheduler | [robfig/cron](https://github.com/robfig/cron) |
| Storage | [BoltDB](https://github.com/etcd-io/bbolt) |
| Timezone | Europe/Moscow (GMT+3) |

---

## orchestrated by
Awet Tsegazeab

## License

MIT
# trueLaunchDetector

A real-time missile and rocket alert bot for WhatsApp, powered by the Israeli Home Front Command (Pikud HaOref) API, OpenAI GPT, and Twilio.

Users subscribe via WhatsApp and receive instant alerts. The bot understands Hebrew and English, prioritizes alerts based on the user's location, and can auto-translate Hebrew area names.

---

## Architecture

```
main.go
 ├── Starts background AlertPoller (goroutine)
 └── Starts Gin HTTP server

AlertPoller (api/alerts.go)
 ├── Polls oref.org.il every 2 seconds
 ├── Deduplicates alerts by ID (10 min TTL)
 └── Broadcasts new alerts to all subscribers concurrently

Gin HTTP Server (api/ginapi.go)
 ├── POST /webhook  — Twilio WhatsApp webhook
 ├── POST /chat     — HTTP endpoint for testing
 └── GET  /health   — Health check

WhatsApp Webhook (api/twilio.go)
 └── Extracts phone number → passes message to GPT

GPT Intent Router (api/gptapi.go)
 ├── Parses Hebrew/English user messages into structured intents
 └── Routes to: subscribe, unsubscribe, set_location,
               set_language, current_alerts, translate, help

SubscriberStore (api/subscribers.go)
 └── In-memory map: phone → { location, language }
```

**Data source:** `https://www.oref.org.il/WarningMessages/alert/alerts.json`

---

## Prerequisites

- Go 1.22+
- Twilio account with a WhatsApp-enabled number
- OpenAI API key
- A public URL for the Twilio webhook (e.g. via [ngrok](https://ngrok.com))

---

## Setup

### 1. Clone the repository

```sh
git clone https://github.com/tamir-liebermann/trueLaunchDetector.git
cd trueLaunchDetector
```

### 2. Create a `.env` file

```env
TWILIO_ACC_SID=your_twilio_account_sid
TWILIO_AUTH=your_twilio_auth_token
TWILIO_PHONE_NUM=whatsapp:+14155238886
OPENAI_API_KEY=your_openai_api_key
APP_WEBHOOK_URL=https://your-public-url.ngrok.io/webhook
```

> `TWILIO_PHONE_NUM` must be prefixed with `whatsapp:` for the WhatsApp sandbox.

### 3. Install dependencies

```sh
go mod tidy
```

### 4. Run the server

```sh
go run main.go
```

The server starts on port `5252` by default. Override with the `PORT` env var.

### 5. Expose the webhook (local development)

```sh
ngrok http 5252
```

Copy the HTTPS URL ngrok gives you, append `/webhook`, and paste it into your Twilio WhatsApp sandbox webhook setting.

---

## Connecting via WhatsApp

1. Open WhatsApp and send a message to your Twilio sandbox number.
2. If using the Twilio sandbox, first join it by sending the sandbox join code (e.g. `join <your-code>`).
3. Start interacting with the bot.

---

## Bot Commands

The bot understands natural language in Hebrew or English.

| What to say | What it does |
|---|---|
| `subscribe` | Start receiving missile alerts |
| `unsubscribe` | Stop receiving alerts |
| `my location is Tel Aviv` | Prioritize alerts for your area (🚨 vs 📢) |
| `my location is תל אביב` | Hebrew city names work too |
| `switch to English` | Auto-translate alert area names to English |
| `עברית` / `switch to Hebrew` | Revert to Hebrew alerts |
| `any alerts?` | Show the last 5 alerts |
| `what's happening?` | Same as above |
| `translate` | Translate the most recent alert to English |
| `help` | Show the command list |

---

## Alert Format

**Hebrew (default):**
```
🚨 *YOUR AREA*
*ירי רקטות וטילים*
אזורים: תל אביב - מרכז העיר
היכנסו למרחב המוגן ושהו בו 10 דקות
```

**English:**
```
🚨 *YOUR AREA*
*Missile/Rocket Alert*
Areas: Tel Aviv - City Center
Enter a protected space and remain for 10 minutes.
```

Alerts not matching your set location use the 📢 prefix instead.

---

## How It Works

1. The `AlertPoller` fetches the Oref API every 2 seconds.
2. Each alert has a unique ID — seen IDs are cached for 10 minutes to prevent duplicate broadcasts.
3. When a new alert is detected, it is broadcast to every subscriber in a separate goroutine.
4. Before sending, the subscriber's location is checked against the alert's area list:
   - Match → 🚨 prefix (urgent)
   - No match → 📢 prefix (informational)
5. If the subscriber's language is set to English, the Hebrew area names are translated via GPT before sending.
6. Incoming WhatsApp messages are parsed by GPT into structured intents, then handled locally (no GPT call for the actual action).

> **Note:** Subscribers are stored in memory. Restarting the server clears all subscriptions.

---

## Deployment

The app is stateless and runs as a single binary. It can be deployed to any platform that supports Go:

- **Docker:** a `Dockerfile.gobank` is included — update it to remove the DB-related steps.
- **GCP App Engine:** an `app.yaml` is included, set your env vars in GCP Secret Manager or directly in `app.yaml`.
- **Railway / Render / Fly.io:** set the env vars in the platform dashboard and deploy.

---

## Contact

tamirlieb2@gmail.com

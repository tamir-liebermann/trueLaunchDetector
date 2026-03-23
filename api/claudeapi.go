package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/gin-gonic/gin"
	"github.com/tamir-liebermann/gobank/env"
)

const (
	SUBSCRIBE_INTENT      = "subscribe"
	UNSUBSCRIBE_INTENT    = "unsubscribe"
	SET_LOCATION_INTENT   = "set_location"
	SET_LANGUAGE_INTENT   = "set_language"
	CURRENT_ALERTS_INTENT = "current_alerts"
	TRANSLATE_INTENT      = "translate"
	HELP_INTENT           = "help"
	CHAT_INTENT           = "chat"
	SUMMARY_INTENT        = "summary"
)

var systemPrompt = `You are an Israeli missile/rocket alert chatbot. Users talk to you via WhatsApp.
Parse the user's message (Hebrew or English) and return ONLY a valid JSON object — no extra text.

Available intents:

subscribe → user wants to receive alerts
{"intent":"subscribe"}

unsubscribe → user wants to stop receiving alerts
{"intent":"unsubscribe"}

set_location → user tells you their city
{"intent":"set_location","body":{"location":"<city as given>"}}

set_language → user wants English or Hebrew responses
{"intent":"set_language","body":{"language":"en"}}
or
{"intent":"set_language","body":{"language":"he"}}

current_alerts → user asks what is happening now / recent alerts
{"intent":"current_alerts"}

translate → user wants the last alert translated to English
{"intent":"translate"}

help → user asks for help or list of commands
{"intent":"help"}

summary → user wants a summary or overview of what happened in the last 12 hours / today / recently
{"intent":"summary"}

chat → user is asking a question or having a conversation about missile alerts, rocket attacks, the security situation in Israel, shelters, safe rooms, Home Front Command instructions, or related geopolitical topics
{"intent":"chat","body":{"message":"<the original user message>"}}

If the message does not match any of the above intents, default to chat.
Return ONLY the JSON. No markdown, no explanation.`

var chatSystemPrompt = `You are a helpful assistant specializing in Israeli security and missile alerts.
You answer questions in the same language the user writes in (Hebrew or English).
You ONLY discuss topics related to: rocket/missile alerts in Israel, the Home Front Command (פיקוד העורף), safe rooms, shelter instructions, the geopolitical and security situation in Israel and the Middle East, and related topics.
If the user asks about anything unrelated, politely decline and redirect them to security-related topics.
Keep answers concise and suitable for WhatsApp (plain text, no markdown headers).`

func claudeClient() *anthropic.Client {
	spec := env.New()
	client := anthropic.NewClient(option.WithAPIKey(spec.AnthropicApiKey))
	return &client
}

func claudeComplete(systemMsg, userMsg string) (string, error) {
	client := claudeClient()
	msg, err := client.Messages.New(context.Background(), anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeHaiku4_5,
		MaxTokens: 256,
		System: []anthropic.TextBlockParam{
			{Text: systemMsg},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userMsg)),
		},
	})
	if err != nil {
		return "", err
	}
	return msg.Content[0].Text, nil
}

func (api *ApiManager) handleChatGPTRequest(ctx *gin.Context, userInput, phone string) string {
	text, err := claudeComplete(systemPrompt, userInput)
	if err != nil {
		return "Sorry, I couldn't process that. Send 'help' for available commands."
	}

	// Strip markdown code fences if model wrapped the JSON
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```") {
		lines := strings.Split(text, "\n")
		if len(lines) > 2 {
			text = strings.Join(lines[1:len(lines)-1], "\n")
		}
	}

	var req GenericRequest
	if err := json.Unmarshal([]byte(text), &req); err != nil {
		return "I didn't understand that. Send 'help' to see what I can do."
	}

	return api.handleAlertIntent(req, phone)
}

func (api *ApiManager) handleAlertIntent(req GenericRequest, phone string) string {
	switch req.Intent {
	case SUBSCRIBE_INTENT:
		api.subscribers.Add(phone)
		return "✅ Subscribed to missile alerts!\nSend 'my location is <city>' to get your area prioritized. 🚨"

	case UNSUBSCRIBE_INTENT:
		api.subscribers.Remove(phone)
		return "✅ You've been unsubscribed."

	case SET_LOCATION_INTENT:
		location, _ := req.Body["location"].(string)
		if location == "" {
			return "Please tell me your city. Example: 'my location is Tel Aviv'"
		}
		api.subscribers.SetLocation(phone, location)
		return fmt.Sprintf("📍 Location set to: *%s*\nAlerts in your area will be marked 🚨", location)

	case SET_LANGUAGE_INTENT:
		lang, _ := req.Body["language"].(string)
		if lang != "en" && lang != "he" {
			lang = "en"
		}
		api.subscribers.SetLanguage(phone, lang)
		if lang == "en" {
			return "🌐 Language set to English. Alerts will be auto-translated."
		}
		return "🌐 שפה הוחלפה לעברית."

	case CURRENT_ALERTS_INTENT:
		return api.formatRecentAlertsForUser(phone)

	case TRANSLATE_INTENT:
		return api.translateLastAlertForUser()

	case HELP_INTENT:
		return helpMessage()

	case SUMMARY_INTENT:
		return api.buildSituationSummary()

	case CHAT_INTENT:
		msg, _ := req.Body["message"].(string)
		if msg == "" {
			return "I didn't understand that.\n\n" + helpMessage()
		}
		reply, err := claudeComplete(chatSystemPrompt, msg)
		if err != nil {
			return "Sorry, couldn't process that right now."
		}
		return reply

	default:
		return "I didn't understand that.\n\n" + helpMessage()
	}
}

func (api *ApiManager) formatRecentAlertsForUser(phone string) string {
	alerts := api.poller.RecentAlerts()
	if len(alerts) == 0 {
		return "✅ No active alerts right now."
	}

	sub, subscribed := api.subscribers.Get(phone)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🔔 *%d recent alert(s):*\n\n", min(len(alerts), 5)))

	for i, alert := range alerts {
		if i >= 5 {
			break
		}
		areas := strings.Join(alert.Data, ", ")
		prefix := "📢"
		if subscribed && sub.Location != "" {
			for _, area := range alert.Data {
				if strings.Contains(area, sub.Location) || strings.Contains(sub.Location, area) {
					prefix = "🚨"
					break
				}
			}
		}
		sb.WriteString(fmt.Sprintf("%s *%s*\nאזורים: %s\n\n", prefix, alert.Title, areas))
	}
	return sb.String()
}

func (api *ApiManager) buildSituationSummary() string {
	alerts := api.poller.History()
	if len(alerts) == 0 {
		return "✅ No alerts recorded in the last 12 hours since the server started."
	}

	var sb strings.Builder
	for _, a := range alerts {
		sb.WriteString(fmt.Sprintf("%s — areas: %s\n", a.Title, strings.Join(a.Data, ", ")))
	}

	prompt := fmt.Sprintf(
		"Here are %d missile/rocket alerts recorded in Israel in the last 12 hours:\n\n%s\nWrite a concise situation summary (4-6 sentences). Include: total alerts, most affected areas, threat types, overall picture. Plain text only.",
		len(alerts), sb.String(),
	)

	summary, err := claudeComplete(chatSystemPrompt, prompt)
	if err != nil {
		return "Sorry, couldn't generate a summary right now."
	}
	return fmt.Sprintf("📊 *Situation Summary — Last 12 Hours*\n\n%s", summary)
}

func (api *ApiManager) translateLastAlertForUser() string {
	alerts := api.poller.RecentAlerts()
	if len(alerts) == 0 {
		return "No recent alerts to translate."
	}
	translated, err := translateAreas(alerts[0].Data)
	if err != nil {
		return "Sorry, couldn't translate right now."
	}
	return fmt.Sprintf("🌐 *Translation*\nAreas: %s\nEnter a protected space and remain for 10 minutes.", strings.Join(translated, ", "))
}

func helpMessage() string {
	return `*Available commands:*
• *subscribe* – Start receiving alerts
• *unsubscribe* – Stop receiving alerts
• *my location is <city>* – Prioritize alerts for your area 🚨
• *switch to English / עברית* – Change alert language
• *any alerts?* – See recent alerts
• *translate* – Translate last alert to English
• *summary* / *what happened today?* – Situation summary for the last 12 hours`
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (api *ApiManager) handleChatGPTEndpoint(ctx *gin.Context) {
	var chatReq ChatReq
	if err := ctx.ShouldBindJSON(&chatReq); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	response := api.handleChatGPTRequest(ctx, chatReq.UserText, "")
	ctx.JSON(http.StatusOK, gin.H{"response": response})
}

package api

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tamir-liebermann/gobank/env"
	"github.com/twilio/twilio-go/client"
)

var phoneRegexp = regexp.MustCompile(`^\+\d{1,3}[-.\s]?\d{1,4}[-.\s]?\d{1,4}[-.\s]?\d{1,9}$`)

func (api *ApiManager) sendWhatsAppMessage(to, message string) error {
	spec := env.New()
	urlStr := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", spec.TwilioAccSid)

	for i, part := range splitMessage(message, 1600) {
		data := url.Values{}
		data.Set("To", to)
		data.Set("From", spec.TwilioPhoneNum)
		data.Set("Body", part)

		req, err := http.NewRequest("POST", urlStr, strings.NewReader(data.Encode()))
		if err != nil {
			return err
		}
		req.SetBasicAuth(spec.TwilioAccSid, spec.TwilioAuth)
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

		resp, err := (&http.Client{}).Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("twilio error (part %d): %s", i+1, string(body))
		}
	}
	return nil
}

func splitMessage(msg string, maxLen int) []string {
	var parts []string
	for i := 0; i < len(msg); i += maxLen {
		end := i + maxLen
		if end > len(msg) {
			end = len(msg)
		}
		parts = append(parts, msg[i:end])
	}
	return parts
}

func (api *ApiManager) handleTwilioWebhook(ctx *gin.Context) {
	var req TwilioReq
	if err := ctx.ShouldBind(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	phone := strings.TrimPrefix(req.From, "whatsapp:")
	if !phoneRegexp.MatchString(phone) {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid phone number"})
		return
	}

	response := api.handleChatGPTRequest(ctx, strings.TrimSpace(req.Body), phone)

	if err := api.sendWhatsAppMessage(req.From, response); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "ok"})
}

func validateTwilioRequest(ctx *gin.Context) bool {
	spec := env.New()
	validator := client.NewRequestValidator(spec.TwilioAuth)
	signature := ctx.GetHeader("X-Twilio-Signature")
	if signature == "" {
		return false
	}
	params := make(map[string]string)
	for key, values := range ctx.Request.PostForm {
		if len(values) > 0 {
			params[key] = values[0]
		}
	}
	return validator.Validate(spec.AppWebhookUrl, params, signature)
}

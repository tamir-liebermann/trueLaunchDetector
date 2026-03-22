package api

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/tamir-liebermann/gobank/env"
	"github.com/twilio/twilio-go"
)

type ApiManager struct {
	twilioClient *twilio.RestClient
	subscribers  *SubscriberStore
	poller       *AlertPoller
}

func NewApiManager() *ApiManager {
	spec := env.New()
	twilioClient := twilio.NewRestClientWithParams(twilio.ClientParams{
		Username: spec.TwilioAccSid,
		Password: spec.TwilioAuth,
	})
	return &ApiManager{
		twilioClient: twilioClient,
		subscribers:  NewSubscriberStore(),
		poller:       NewAlertPoller(),
	}
}

func (api *ApiManager) RegisterRoutes(server *gin.Engine) {
	server.GET("/health", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	server.POST("/webhook", api.handleTwilioWebhook)
	server.POST("/chat", api.handleChatGPTEndpoint)
}

func (api *ApiManager) StartAlertPoller() {
	go api.poller.Start(api.subscribers, func(sub *Subscriber, alert OrefAlert) {
		api.broadcastAlert(sub, alert)
	})
}

func (api *ApiManager) Run() {
	server := gin.Default()
	api.RegisterRoutes(server)

	port := os.Getenv("PORT")
	if port == "" {
		port = "5252"
	}
	server.Run(":" + port)
}

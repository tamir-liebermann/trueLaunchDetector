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
	server.POST("/test/alert", api.handleTestAlert)
}

// handleTestAlert injects a fake alert into the pipeline for testing.
func (api *ApiManager) handleTestAlert(ctx *gin.Context) {
	var alert OrefAlert
	if err := ctx.ShouldBindJSON(&alert); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if alert.ID == "" {
		alert.ID = "test-" + alert.Title
	}
	api.poller.storeRecent(alert)
	subs := api.subscribers.All()
	for _, sub := range subs {
		go api.broadcastAlert(sub, alert)
	}
	ctx.JSON(http.StatusOK, gin.H{"injected": true, "subscribers": len(subs)})
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

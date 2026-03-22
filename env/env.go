package env

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

var spec Specification

type Specification struct {
	TwilioAuth     string
	TwilioAccSid   string
	AnthropicApiKey string
	TwilioPhoneNum string
	AppWebhookUrl  string
	JwtSecret      string // kept for utils package compatibility
	MongoSecret    string // kept for db package compatibility
}

func New() *Specification {
	godotenv.Load()

	spec = Specification{
		TwilioAuth:     getEnvVar("TWILIO_AUTH"),
		TwilioAccSid:   getEnvVar("TWILIO_ACC_SID"),
		AnthropicApiKey: getEnvVar("ANTHROPIC_API_KEY"),
		TwilioPhoneNum: getEnvVar("TWILIO_PHONE_NUM"),
		AppWebhookUrl:  getEnvVar("APP_WEBHOOK_URL"),
		JwtSecret:      os.Getenv("JWT_SECRET"),   // optional
		MongoSecret:    os.Getenv("MONGODB_URI"),  // optional
	}
	return &spec
}

func getEnvVar(varName string) string {
	envVar := os.Getenv(varName)
	if envVar == "" {
		log.Panicln(varName, " environment variable is not set.")
	}
	return envVar
}

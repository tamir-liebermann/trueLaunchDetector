package api

// OrefAlert is the alert structure returned by the Pikud HaOref API
type OrefAlert struct {
	ID    string   `json:"id"`
	Cat   string   `json:"cat"`
	Title string   `json:"title"`
	Data  []string `json:"data"` // area names in Hebrew
	Desc  string   `json:"desc"`
}

type TwilioReq struct {
	From string `form:"From" json:"From"`
	Body string `form:"Body" json:"Body"`
}

type ChatReq struct {
	UserText string `json:"user_text"`
}

type GenericRequest struct {
	Intent string                 `json:"intent"`
	Body   map[string]interface{} `json:"body"`
}

type ErrorResponse struct {
	Message string `json:"message"`
}

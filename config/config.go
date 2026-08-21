package config

import (
	_ "embed"
	"encoding/json"
)

//go:embed default.json
var defaultConfig []byte



//go:embed gemini-key.txt
var embeddedGeminiKey []byte


type Config struct {
	Address            string `json:"address"`
	GoogleRedirectURL  string `json:"google_redirect_url"`
	MeetLookbackMonths int    `json:"meet_lookback_months"`
	GeminiModel        string `json:"gemini_model"`
}
func NewConfig()(*Config,error){
	var cfg Config
	if err := json.Unmarshal(defaultConfig, &cfg); err != nil {
		return nil,err
	}
	return  &cfg,nil
}

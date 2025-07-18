package ai

import (
	"github.com/Builder-Lawyers/builder-backend/pkg/env"
	"strconv"
)

type OpenAIConfig struct {
	apiKey    string
	model     string
	maxTokens int64
}

func NewOpenAIConfig() OpenAIConfig {
	maxTokens, err := strconv.Atoi(env.GetEnv("OPENAI_TOKENS", "600"))
	if err != nil {
		maxTokens = 600
	}
	return OpenAIConfig{
		apiKey:    env.GetEnv("OPENAI_KEY", ""),
		model:     env.GetEnv("OPENAI_MODEL", ""),
		maxTokens: int64(maxTokens),
	}
}

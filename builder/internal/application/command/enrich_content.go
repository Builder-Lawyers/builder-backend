package command

import (
	ai "builder/internal/infra/client/openai"
	"builder/internal/presentation/rest"
)

type EnrichContent struct {
	aiClient *ai.OpenAIClient
}

func NewEnrichContent(client ai.OpenAIClient) EnrichContent {
	return EnrichContent{
		&client,
	}
}

func (c EnrichContent) Execute(req rest.EnrichContentRequest) (string, error) {
	enriched, err := c.aiClient.EnrichContent(req.Content)
	if err != nil {
		return "", err
	}

	return enriched, nil
}

package ai

import (
	"context"

	"github.com/Builder-Lawyers/builder-backend/internal/application/dto"
	ai "github.com/Builder-Lawyers/builder-backend/internal/infra/client/openai"
)

type EnrichContent struct {
	aiClient *ai.OpenAIClient
}

func NewEnrichContent(client *ai.OpenAIClient) *EnrichContent {
	return &EnrichContent{
		client,
	}
}

func (c EnrichContent) Execute(ctx context.Context, req *dto.EnrichContentRequest) (string, error) {
	enriched, err := c.aiClient.EnrichContent(req.Content)
	if err != nil {
		return "", err
	}

	return enriched, nil
}

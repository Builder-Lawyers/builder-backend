package ai

import (
	"context"
	"fmt"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/shared"
)

type OpenAIClient struct {
	cfg    OpenAIConfig
	client openai.Client
}

func NewOpenAIClient(config OpenAIConfig) *OpenAIClient {
	return &OpenAIClient{
		config,
		openai.NewClient(option.WithAPIKey(config.apiKey)),
	}
}

func (c *OpenAIClient) EnrichContent(req string) (string, error) {

	messages := make([]openai.ChatCompletionMessageParamUnion, 0)
	messages = append(messages, openai.ChatCompletionMessageParamUnion{
		OfUser: &openai.ChatCompletionUserMessageParam{
			Role: "user",
			Name: param.Opt[string]{Value: "idk why"},
			Content: openai.ChatCompletionUserMessageParamContentUnion{
				OfString: param.Opt[string]{Value: req},
			},
		},
	})

	chatCompletion, err := c.client.Chat.Completions.New(context.TODO(), openai.ChatCompletionNewParams{
		Model:               c.cfg.model,
		Messages:            messages,
		MaxCompletionTokens: param.Opt[int64]{Value: c.cfg.maxTokens},
		N:                   param.Opt[int64]{Value: 1},
		Temperature:         param.Opt[float64]{Value: 0.8},
		ReasoningEffort:     shared.ReasoningEffortMedium,
	})
	if err != nil {
		fmt.Println(err)
		return "", err
	}

	return chatCompletion.Choices[0].Message.Content, nil
}

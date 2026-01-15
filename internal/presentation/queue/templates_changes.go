package queue

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"time"

	"github.com/Builder-Lawyers/builder-backend/internal/application/commands/template"
	"github.com/Builder-Lawyers/builder-backend/internal/application/dto"
	"github.com/Builder-Lawyers/builder-backend/pkg/env"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

type TemplateChangesPoller struct {
	client  *sqs.Client
	cfg     TemplateChangesConfig
	handler *template.RebuildTemplate
	stop    chan struct{}
}

type TemplateChangesConfig struct {
	Enabled   bool
	SqsURL    string
	SqsRegion string
}

func NewTemplateChangesConfig() TemplateChangesConfig {
	return TemplateChangesConfig{
		Enabled:   os.Getenv("TEMPLATES_SQS_ENABLED") == "true",
		SqsURL:    os.Getenv("TEMPLATES_SQS_URL"),
		SqsRegion: env.GetEnv("TEMPLATES_SQS_REGION", "us-east-1"),
	}
}

type TemplatesChanges struct {
	Commit    string   `json:"commit"`
	Templates []string `json:"templates"`
}

func NewTemplateChangesPoller(client *sqs.Client, cfg TemplateChangesConfig, handler *template.RebuildTemplate) *TemplateChangesPoller {
	return &TemplateChangesPoller{client: client, cfg: cfg, stop: make(chan struct{}), handler: handler}
}

func (p *TemplateChangesPoller) Start() {
	slog.Info("Starting poll of TemplateChangesPoller...")
	ctx := context.Background()

	for {
		select {
		case <-p.stop:
			slog.Info("Stopping TemplateChangesPoller loop")
			return
		default:
			slog.Debug("Template Changes poll")
			out, err := p.client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
				QueueUrl:            aws.String(p.cfg.SqsURL),
				MaxNumberOfMessages: 10,
				WaitTimeSeconds:     20,
				VisibilityTimeout:   30,
			})
			if err != nil {
				slog.Info("err receiving from queue", "err", err)
				time.Sleep(time.Second)
				continue
			}
			if len(out.Messages) == 0 {
				continue
			}

			// TODO: consume all messages and collect templates in set before processing
			processedMessages := make([]types.DeleteMessageBatchRequestEntry, len(out.Messages))
			changedTemplates := make(map[string]struct{})
			for i, m := range out.Messages {
				slog.Debug("msg received from queue", "msg", *m.Body)

				var templatesChanges TemplatesChanges
				err = json.Unmarshal([]byte(*m.Body), &templatesChanges)
				if err != nil {
					slog.Error("err unmarshalling msg", "id", m.MessageId, "err", err)
				}

				for _, templateToChange := range templatesChanges.Templates {
					if _, ok := changedTemplates[templateToChange]; !ok {
						changedTemplates[templateToChange] = struct{}{}
					}
				}

				processedMessages[i] = types.DeleteMessageBatchRequestEntry{
					Id:            m.MessageId,
					ReceiptHandle: m.ReceiptHandle,
				}
			}

			for changedTemplate, _ := range changedTemplates {
				err = p.handler.Execute(ctx, &dto.RebuildTemplatesRequest{Name: &changedTemplate})
				if err != nil {
					slog.Error("err updating template", "template", changedTemplate, "err", err)
				}
			}

			_, err = p.client.DeleteMessageBatch(ctx, &sqs.DeleteMessageBatchInput{
				QueueUrl: aws.String(p.cfg.SqsURL),
				Entries:  processedMessages,
			})
			if err != nil {
				slog.Error("err deleting message", "err", err)
			}
		}
	}
}

func (p *TemplateChangesPoller) Stop() {
	p.stop <- struct{}{}
}

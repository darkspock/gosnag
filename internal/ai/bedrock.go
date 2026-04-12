package ai

import (
	"context"
	"fmt"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	gosnagconfig "github.com/darkspock/gosnag/internal/config"
)

type bedrockProvider struct {
	client  *bedrockruntime.Client
	modelID string
}

func newBedrockProvider(cfg *gosnagconfig.Config) *bedrockProvider {
	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(cfg.AIBedrockRegion),
	)
	if err != nil {
		return &bedrockProvider{} // will fail on Chat call
	}

	client := bedrockruntime.NewFromConfig(awsCfg)
	modelID := cfg.AIBedrockModelID
	if modelID == "" {
		modelID = "anthropic.claude-3-haiku-20240307-v1:0"
	}

	return &bedrockProvider{
		client:  client,
		modelID: modelID,
	}
}

func (p *bedrockProvider) Name() string { return "bedrock" }

func (p *bedrockProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	if p.client == nil {
		return nil, fmt.Errorf("bedrock: client not initialized")
	}

	msgs := make([]types.Message, 0, len(req.Messages))
	for _, m := range req.Messages {
		role := types.ConversationRoleUser
		if m.Role == "assistant" {
			role = types.ConversationRoleAssistant
		}
		msgs = append(msgs, types.Message{
			Role: role,
			Content: []types.ContentBlock{
				&types.ContentBlockMemberText{Value: m.Content},
			},
		})
	}

	input := &bedrockruntime.ConverseInput{
		ModelId:  &p.modelID,
		Messages: msgs,
	}

	if req.SystemPrompt != "" {
		systemPrompt := req.SystemPrompt
		if req.JSON {
			systemPrompt += "\n\nYou MUST respond with valid JSON only. No markdown, no explanation outside the JSON."
		}
		input.System = []types.SystemContentBlock{
			&types.SystemContentBlockMemberText{Value: systemPrompt},
		}
	}

	if req.MaxTokens > 0 {
		maxTokens := int32(req.MaxTokens)
		input.InferenceConfig = &types.InferenceConfiguration{
			MaxTokens:   &maxTokens,
			Temperature: float32Ptr(float32(req.Temperature)),
		}
	}

	output, err := p.client.Converse(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("bedrock converse: %w", err)
	}

	var content string
	if msg, ok := output.Output.(*types.ConverseOutputMemberMessage); ok {
		for _, block := range msg.Value.Content {
			if text, ok := block.(*types.ContentBlockMemberText); ok {
				content = text.Value
				break
			}
		}
	}

	var inputTokens, outputTokens int
	if output.Usage != nil {
		inputTokens = int(*output.Usage.InputTokens)
		outputTokens = int(*output.Usage.OutputTokens)
	}

	return &ChatResponse{
		Content:      content,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
	}, nil
}

func float32Ptr(f float32) *float32 { return &f }

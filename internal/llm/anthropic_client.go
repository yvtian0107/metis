package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

type anthropicClient struct {
	client anthropic.Client
}

func newAnthropicClient(baseURL, apiKey string) *anthropicClient {
	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
		// Set HTTP client with 5min timeout; caller's context deadline is the primary timeout control
		option.WithHTTPClient(&http.Client{
			Timeout: 300 * time.Second,
		}),
	}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	return &anthropicClient{client: anthropic.NewClient(opts...)}
}

func (c *anthropicClient) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	jsonMode := req.ResponseFormat != nil && (req.ResponseFormat.Type == "json_object" || req.ResponseFormat.Type == "json_schema")
	params := c.buildParams(req)
	msg, err := c.client.Messages.New(ctx, params)
	if err != nil {
		return nil, err
	}

	result := &ChatResponse{
		Usage: Usage{
			InputTokens:  int(msg.Usage.InputTokens),
			OutputTokens: int(msg.Usage.OutputTokens),
		},
	}

	for _, block := range msg.Content {
		switch block.Type {
		case "text":
			result.Content += block.Text
		case "tool_use":
			args, _ := json.Marshal(block.Input)
			result.ToolCalls = append(result.ToolCalls, ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: string(args),
			})
		}
	}

	// Prepend "{" for JSON mode since the assistant prefill is not included in the response
	if jsonMode && len(req.Tools) == 0 && result.Content != "" && !strings.HasPrefix(result.Content, "{") {
		result.Content = "{" + result.Content
	}

	return result, nil
}

func (c *anthropicClient) ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error) {
	params := c.buildParams(req)
	stream := c.client.Messages.NewStreaming(ctx, params)

	ch := make(chan StreamEvent, 32)
	go func() {
		defer close(ch)

		var usage *Usage
		// Accumulate tool calls
		toolCalls := map[int]*ToolCall{}

		for stream.Next() {
			event := stream.Current()

			switch event.Type {
			case "content_block_start":
				if event.ContentBlock.Type == "tool_use" {
					idx := int(event.Index)
					toolCalls[idx] = &ToolCall{
						ID:   event.ContentBlock.ID,
						Name: event.ContentBlock.Name,
					}
				}
			case "content_block_delta":
				switch event.Delta.Type {
				case "text_delta":
					ch <- StreamEvent{Type: "content_delta", Content: event.Delta.Text}
				case "input_json_delta":
					idx := int(event.Index)
					if tc, ok := toolCalls[idx]; ok {
						tc.Arguments += event.Delta.PartialJSON
					}
				}
			case "content_block_stop":
				idx := int(event.Index)
				if tc, ok := toolCalls[idx]; ok {
					ch <- StreamEvent{Type: "tool_call", ToolCall: &ToolCall{
						ID:        tc.ID,
						Name:      tc.Name,
						Arguments: tc.Arguments,
					}}
					delete(toolCalls, idx)
				}
			case "message_delta":
				usage = &Usage{
					OutputTokens: int(event.Usage.OutputTokens),
				}
			case "message_start":
				if usage == nil {
					usage = &Usage{}
				}
				usage.InputTokens = int(event.Message.Usage.InputTokens)
			}
		}

		if err := stream.Err(); err != nil {
			ch <- StreamEvent{Type: "error", Error: err.Error()}
			return
		}

		ch <- StreamEvent{Type: "done", Usage: usage}
	}()

	return ch, nil
}

func (c *anthropicClient) Embedding(_ context.Context, _ EmbeddingRequest) (*EmbeddingResponse, error) {
	return nil, ErrNotSupported
}

// parseBase64DataURL parses a base64 data URL and returns the media type and base64 data.
// Format: data:image/png;base64,xxxxx
func parseBase64DataURL(url string) (anthropic.Base64ImageSourceMediaType, string) {
	const prefix = "data:image/"
	if !strings.HasPrefix(url, prefix) {
		return "", ""
	}

	// Find the comma that separates metadata from data
	idx := strings.Index(url, ",")
	if idx == -1 {
		return "", ""
	}

	// Extract media type from: image/png;base64
	meta := url[len(prefix):idx]
	parts := strings.Split(meta, ";")
	if len(parts) < 1 {
		return "", ""
	}

	var mediaType anthropic.Base64ImageSourceMediaType
	switch parts[0] {
	case "png":
		mediaType = anthropic.Base64ImageSourceMediaTypeImagePNG
	case "jpeg", "jpg":
		mediaType = anthropic.Base64ImageSourceMediaTypeImageJPEG
	case "gif":
		mediaType = anthropic.Base64ImageSourceMediaTypeImageGIF
	case "webp":
		mediaType = anthropic.Base64ImageSourceMediaTypeImageWebP
	default:
		return "", ""
	}

	data := url[idx+1:]
	return mediaType, data
}

func (c *anthropicClient) buildParams(req ChatRequest) anthropic.MessageNewParams {
	maxTokens := int64(req.MaxTokens)
	if maxTokens <= 0 {
		maxTokens = 4096
	}

	// Handle ResponseFormat for Anthropic (no native response_format support)
	// json_object: append system prompt constraint + assistant prefill "{"
	jsonMode := req.ResponseFormat != nil && (req.ResponseFormat.Type == "json_object" || req.ResponseFormat.Type == "json_schema")

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(req.Model),
		MaxTokens: maxTokens,
	}

	for _, msg := range req.Messages {
		switch msg.Role {
		case RoleSystem:
			systemText := msg.Content
			if jsonMode {
				systemText += "\n\nIMPORTANT: You must respond with valid JSON only. Do not include any text before or after the JSON object."
			}
			params.System = []anthropic.TextBlockParam{
				{Text: systemText},
			}
		case RoleUser:
			// Build content blocks for multimodal support
			var blocks []anthropic.ContentBlockParamUnion
			if msg.Content != "" {
				blocks = append(blocks, anthropic.NewTextBlock(msg.Content))
			}
			for _, img := range msg.Images {
				// Parse base64 data URL: data:image/png;base64,xxxxx
				mediaType, data := parseBase64DataURL(img)
				if data != "" {
					blocks = append(blocks, anthropic.NewImageBlock(
						anthropic.Base64ImageSourceParam{
							Data:      data,
							MediaType: mediaType,
						},
					))
				}
			}
			params.Messages = append(params.Messages, anthropic.NewUserMessage(blocks...))
		case RoleAssistant:
			if len(msg.ToolCalls) > 0 {
				var blocks []anthropic.ContentBlockParamUnion
				if msg.Content != "" {
					blocks = append(blocks, anthropic.NewTextBlock(msg.Content))
				}
				for _, tc := range msg.ToolCalls {
					var input any
					json.Unmarshal([]byte(tc.Arguments), &input)
					blocks = append(blocks, anthropic.NewToolUseBlock(tc.ID, input, tc.Name))
				}
				params.Messages = append(params.Messages, anthropic.NewAssistantMessage(blocks...))
			} else {
				params.Messages = append(params.Messages, anthropic.NewAssistantMessage(
					anthropic.NewTextBlock(msg.Content),
				))
			}
		case RoleTool:
			params.Messages = append(params.Messages, anthropic.MessageParam{
				Role: anthropic.MessageParamRoleUser,
				Content: []anthropic.ContentBlockParamUnion{
					anthropic.NewToolResultBlock(msg.ToolCallID, msg.Content, false),
				},
			})
		}
	}

	if req.Temperature != nil {
		params.Temperature = anthropic.Float(float64(*req.Temperature))
	}

	for _, tool := range req.Tools {
		inputSchema := anthropic.ToolInputSchemaParam{}
		if tool.Parameters != nil {
			raw, _ := json.Marshal(tool.Parameters)
			json.Unmarshal(raw, &inputSchema)
		}
		params.Tools = append(params.Tools, anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        tool.Name,
				Description: anthropic.String(tool.Description),
				InputSchema: inputSchema,
			},
		})
	}

	// Assistant prefill for JSON mode (only when no tools — tools use their own structured output)
	if jsonMode && len(req.Tools) == 0 {
		params.Messages = append(params.Messages, anthropic.NewAssistantMessage(
			anthropic.NewTextBlock("{"),
		))
	}

	return params
}

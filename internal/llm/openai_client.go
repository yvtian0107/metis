package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

type openaiClient struct {
	client *openai.Client
}

func newOpenAIClient(baseURL, apiKey string) *openaiClient {
	cfg := openai.DefaultConfig(apiKey)
	if baseURL != "" {
		cfg.BaseURL = baseURL
	}
	// Set HTTP client with 5min timeout; caller's context deadline is the primary timeout control
	cfg.HTTPClient = &http.Client{
		Timeout: 300 * time.Second,
	}
	return &openaiClient{client: openai.NewClientWithConfig(cfg)}
}

func (c *openaiClient) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	oaiReq := c.buildRequest(req)
	resp, err := c.client.CreateChatCompletion(ctx, oaiReq)
	if err != nil {
		return nil, err
	}

	result := &ChatResponse{
		Usage: Usage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
		},
	}

	if len(resp.Choices) > 0 {
		choice := resp.Choices[0]
		result.Content = choice.Message.Content
		for _, tc := range choice.Message.ToolCalls {
			result.ToolCalls = append(result.ToolCalls, ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			})
		}
	}

	return result, nil
}

func (c *openaiClient) ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error) {
	oaiReq := c.buildRequest(req)
	oaiReq.Stream = true
	oaiReq.StreamOptions = &openai.StreamOptions{IncludeUsage: true}

	stream, err := c.client.CreateChatCompletionStream(ctx, oaiReq)
	if err != nil {
		return nil, err
	}

	ch := make(chan StreamEvent, 32)
	go func() {
		defer close(ch)
		defer stream.Close()

		// Accumulate tool calls across chunks
		toolCalls := map[int]*ToolCall{}

		for {
			resp, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					break
				}
				ch <- StreamEvent{Type: "error", Error: err.Error()}
				return
			}

			// Usage chunk (final)
			if resp.Usage != nil && resp.Usage.PromptTokens > 0 {
				ch <- StreamEvent{
					Type: "done",
					Usage: &Usage{
						InputTokens:  resp.Usage.PromptTokens,
						OutputTokens: resp.Usage.CompletionTokens,
					},
				}
				continue
			}

			if len(resp.Choices) == 0 {
				continue
			}

			delta := resp.Choices[0].Delta

			// Content delta
			if delta.Content != "" {
				ch <- StreamEvent{Type: "content_delta", Content: delta.Content}
			}

			// Tool call deltas
			for _, tc := range delta.ToolCalls {
				idx := 0
				if tc.Index != nil {
					idx = *tc.Index
				}
				existing, ok := toolCalls[idx]
				if !ok {
					existing = &ToolCall{ID: tc.ID, Name: tc.Function.Name}
					toolCalls[idx] = existing
				}
				existing.Arguments += tc.Function.Arguments

				// If this chunk has a finish reason of tool_calls, emit accumulated
			}

			if resp.Choices[0].FinishReason == openai.FinishReasonToolCalls {
				for _, tc := range toolCalls {
					ch <- StreamEvent{Type: "tool_call", ToolCall: &ToolCall{
						ID:        tc.ID,
						Name:      tc.Name,
						Arguments: tc.Arguments,
					}}
				}
				toolCalls = map[int]*ToolCall{}
			}
		}
	}()

	return ch, nil
}

func (c *openaiClient) Embedding(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error) {
	oaiReq := openai.EmbeddingRequest{
		Model: openai.EmbeddingModel(req.Model),
		Input: req.Input,
	}

	resp, err := c.client.CreateEmbeddings(ctx, oaiReq)
	if err != nil {
		return nil, err
	}

	result := &EmbeddingResponse{}
	result.Usage.TotalTokens = resp.Usage.TotalTokens
	for _, emb := range resp.Data {
		result.Embeddings = append(result.Embeddings, emb.Embedding)
	}
	return result, nil
}

func (c *openaiClient) buildRequest(req ChatRequest) openai.ChatCompletionRequest {
	oaiReq := openai.ChatCompletionRequest{
		Model: req.Model,
	}

	for _, msg := range req.Messages {
		// Handle multimodal messages with images
		if len(msg.Images) > 0 {
			parts := make([]openai.ChatMessagePart, 0, len(msg.Images)+1)
			// Add text content
			if msg.Content != "" {
				parts = append(parts, openai.ChatMessagePart{
					Type: openai.ChatMessagePartTypeText,
					Text: msg.Content,
				})
			}
			// Add image content
			for _, img := range msg.Images {
				parts = append(parts, openai.ChatMessagePart{
					Type: openai.ChatMessagePartTypeImageURL,
					ImageURL: &openai.ChatMessageImageURL{
						URL: img,
					},
				})
			}
			oaiReq.Messages = append(oaiReq.Messages, openai.ChatCompletionMessage{
				Role:         msg.Role,
				MultiContent: parts,
				ToolCallID:   msg.ToolCallID,
			})
		} else {
			// Simple text message
			oaiMsg := openai.ChatCompletionMessage{
				Role:       msg.Role,
				Content:    msg.Content,
				ToolCallID: msg.ToolCallID,
			}
			for _, tc := range msg.ToolCalls {
				oaiMsg.ToolCalls = append(oaiMsg.ToolCalls, openai.ToolCall{
					ID:   tc.ID,
					Type: openai.ToolTypeFunction,
					Function: openai.FunctionCall{
						Name:      tc.Name,
						Arguments: tc.Arguments,
					},
				})
			}
			oaiReq.Messages = append(oaiReq.Messages, oaiMsg)
		}
	}

	if req.MaxTokens > 0 {
		oaiReq.MaxCompletionTokens = req.MaxTokens
	}
	if req.Temperature != nil && !openAIModelHasFixedSampling(req.Model) {
		oaiReq.Temperature = *req.Temperature
	}

	if req.ResponseFormat != nil {
		switch req.ResponseFormat.Type {
		case "json_object":
			oaiReq.ResponseFormat = &openai.ChatCompletionResponseFormat{
				Type: openai.ChatCompletionResponseFormatTypeJSONObject,
			}
		case "json_schema":
			// Convert schema to json.RawMessage (implements json.Marshaler)
			schemaRaw, _ := json.Marshal(req.ResponseFormat.Schema)
			oaiReq.ResponseFormat = &openai.ChatCompletionResponseFormat{
				Type: openai.ChatCompletionResponseFormatTypeJSONSchema,
				JSONSchema: &openai.ChatCompletionResponseFormatJSONSchema{
					Name:   "response",
					Schema: json.RawMessage(schemaRaw),
					Strict: true,
				},
			}
		}
	}

	for _, tool := range req.Tools {
		oaiReq.Tools = append(oaiReq.Tools, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
			},
		})
	}

	return oaiReq
}

func openAIModelHasFixedSampling(model string) bool {
	normalized := strings.ToLower(strings.TrimSpace(model))
	return normalized == "gpt-5.4" || strings.HasPrefix(normalized, "gpt-5.4-")
}

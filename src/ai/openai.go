package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"mogenius-operator/src/store"
	"time"

	"github.com/openai/openai-go/v3"
)

var openAiTools = []openai.ChatCompletionToolUnionParam{
	{
		OfFunction: &openai.ChatCompletionFunctionToolParam{
			Function: openai.FunctionDefinitionParam{
				Name:        "get_kubernetes_resources",
				Description: openai.String("Get Kubernetes resources by kind, optionally filtered by name and namespace"),
				Parameters: openai.FunctionParameters{
					"type": "object",
					"properties": map[string]interface{}{
						"apiVersion": map[string]string{
							"type": "string",
						},
						"kind": map[string]string{
							"type": "string",
						},
						"name": map[string]string{
							"type": "string",
						},
						"namespace": map[string]string{
							"type": "string",
						},
					},
					"required": []string{"kind", "apiVersion"},
				},
			},
		},
	},
}

func (ai *aiManager) processPromptOpenAi(ctx context.Context, model, systemPrompt, prompt string) (*AiResponse, int64, int, string, error) {
	startTime := time.Now()

	client, err := ai.getOpenAIClient(nil)
	if err != nil {
		return nil, 0, int(time.Since(startTime).Milliseconds()), model, err
	}

	params := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(prompt),
			openai.SystemMessage(systemPrompt + "\n You have access to the following tool: get_kubernetes_resources. Use it to retrieve Kubernetes resources as needed to answer the user's question accurately."),
		},
		Model:       model,
		Tools:       openAiTools,
		Temperature: openai.Float(0.1),
	}

	var tokensUsed int64 = 0
	var chatCompletion *openai.ChatCompletion

	// Loop until there are no more tool calls
	for {
		chatCompletion, err = client.Chat.Completions.New(ctx, params)
		if err != nil {
			return nil, tokensUsed, int(time.Since(startTime).Milliseconds()), model, err
		}

		if chatCompletion != nil {
			tokensUsed += chatCompletion.Usage.TotalTokens
		}

		if len(chatCompletion.Choices) == 0 {
			return nil, tokensUsed, int(time.Since(startTime).Milliseconds()), model, fmt.Errorf("no choices returned from AI model")
		}

		// Add the assistant's response to the messages
		params.Messages = append(params.Messages, chatCompletion.Choices[0].Message.ToParam())

		// Check if there are tool calls to process
		if len(chatCompletion.Choices[0].Message.ToolCalls) == 0 {
			ai.logger.Info("No tool calls found, finishing AI processing")
			break
		}

		// Process each tool call
		for _, toolCall := range chatCompletion.Choices[0].Message.ToolCalls {
			ai.logger.Info("Processing tool call", "tool", toolCall.Function.Name)
			if toolCall.Function.Name == "get_kubernetes_resources" {
				// Extract the location from the function call arguments
				var args map[string]interface{}
				err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
				if err != nil {
					return nil, tokensUsed, int(time.Since(startTime).Milliseconds()), model, fmt.Errorf("error unmarshaling tool arguments: %v", err)
				}
				kind := args["kind"].(string)
				apiVersion := args["apiVersion"].(string)
				name := args["name"].(string)
				namespace := args["namespace"].(string)

				ai.logger.Info("Retrieving Kubernetes resources", "apiVersion", apiVersion, "kind", kind, "namespace", namespace, "name", name)
				resources, err := store.GetResource(ai.valkeyClient, apiVersion, kind, namespace, name, ai.logger)
				data := ""
				if err != nil {
					data = fmt.Sprintf("Error retrieving resources: %v", err)
				} else {
					resourceBytes, err := json.MarshalIndent(resources, "", "  ")
					if err != nil {
						data = fmt.Sprintf("Error marshaling resources: %v", err)
					} else {
						data = string(resourceBytes)
					}
				}

				params.Messages = append(params.Messages, openai.ToolMessage(data, toolCall.ID))
			}
		}
		// Continue the loop to get the next response with tool results
	}

	responseText := cleanJSONResponse(chatCompletion.Choices[0].Message.Content)
	responseBytes, removedText, err := extractJSONRobust(responseText)
	ai.logger.Info("Extracted JSON from AI response", "removed_text", removedText)
	if err != nil {
		return nil, tokensUsed, int(time.Since(startTime).Milliseconds()), model, fmt.Errorf("error extracting JSON from AI response: %v\n%s", err, responseText)
	}

	var aiResponse AiResponse
	err = json.Unmarshal(responseBytes, &aiResponse)
	if err != nil {
		return nil, tokensUsed, int(time.Since(startTime).Milliseconds()), model, fmt.Errorf("error unmarshaling AI response: %v\n%s", err, chatCompletion.Choices[0].Message.Content)
	}

	// also return tokens used
	return &aiResponse, tokensUsed, int(time.Since(startTime).Milliseconds()), model, nil
}

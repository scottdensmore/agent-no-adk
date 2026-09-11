package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"sre-triage-agent/internal/tools"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/genai"
)

type Config struct {
	Model     string
	ProjectID string
	Location  string
	APIKey    string
	Client    *genai.Client
}

type ToolDispatcher func(name string, args map[string]any) (any, error)

type AgentEvent struct {
	Type     string         // "functionCall", "functionResponse", "text"
	Name     string         // Tool name (for functionCall/functionResponse)
	Args     map[string]any // Tool args (for functionCall)
	Response map[string]any // Tool response (for functionResponse)
	Text     string         // Text content (for text)
}

type EventCallback func(event AgentEvent)

type Agent struct {
	client     *genai.Client
	ModelName  string
	dispatcher ToolDispatcher
}

func NewAgent(ctx context.Context, cfg Config) (*Agent, error) {
	if cfg.Model == "" {
		cfg.Model = os.Getenv("GEMINI_MODEL")
		if cfg.Model == "" {
			cfg.Model = "gemini-3.8-flash"
		}
	}

	if cfg.Client != nil {
		return &Agent{
			client:     cfg.Client,
			ModelName:  cfg.Model,
			dispatcher: tools.DispatchTool,
		}, nil
	}

	clientCfg := &genai.ClientConfig{}

	// Check if Vertex AI should be used
	project := cfg.ProjectID
	if project == "" {
		project = os.Getenv("GOOGLE_CLOUD_PROJECT")
	}
	if project == "" {
		project = os.Getenv("GCLOUD_PROJECT")
	}
	if project == "" {
		if home, err := os.UserHomeDir(); err == nil {
			adcPath := filepath.Join(home, ".config", "gcloud", "application_default_credentials.json")
			if b, err := os.ReadFile(adcPath); err == nil {
				var adc struct {
					QuotaProjectID string `json:"quota_project_id"`
					ProjectID      string `json:"project_id"`
				}
				if err := json.Unmarshal(b, &adc); err == nil {
					if adc.QuotaProjectID != "" {
						project = adc.QuotaProjectID
					} else if adc.ProjectID != "" {
						project = adc.ProjectID
					}
				}
			}
		}
	}
	location := cfg.Location
	if location == "" {
		location = os.Getenv("GOOGLE_CLOUD_LOCATION")
		if location == "" {
			location = "global"
		}
	}

	apiKey := cfg.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("GEMINI_API_KEY")
	}

	if project != "" || os.Getenv("GOOGLE_GENAI_USE_VERTEXAI") == "true" {
		clientCfg.Backend = genai.BackendVertexAI
		clientCfg.Project = project
		clientCfg.Location = location
		log.Printf("Agent using Vertex AI Backend (project=%s, location=%s)", project, location)
	} else if apiKey != "" {
		clientCfg.Backend = genai.BackendGeminiAPI
		clientCfg.APIKey = apiKey
		log.Println("Agent using Gemini API Backend via API Key")
	} else {
		// Default to Vertex AI with ADC
		clientCfg.Backend = genai.BackendVertexAI
		clientCfg.Location = location
		log.Println("Agent defaulting to Vertex AI Backend with Application Default Credentials")
	}

	client, err := genai.NewClient(ctx, clientCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create genai client: %w", err)
	}

	return &Agent{
		client:     client,
		ModelName:  cfg.Model,
		dispatcher: tools.DispatchTool,
	}, nil
}

func NewAgentWithDispatcher(cfg Config, dispatcher ToolDispatcher) (*Agent, error) {
	if cfg.Model == "" {
		cfg.Model = "gemini-3.8-flash"
	}
	return &Agent{
		client:     cfg.Client,
		ModelName:  cfg.Model,
		dispatcher: dispatcher,
	}, nil
}

func (a *Agent) Invoke(ctx context.Context, userPrompt string, contextID string) (string, error) {
	return a.InvokeWithEvents(ctx, userPrompt, contextID, nil)
}

func (a *Agent) InvokeWithEvents(ctx context.Context, userPrompt string, contextID string, onEvent EventCallback) (string, error) {
	if a.client == nil {
		msg := "Agent client is uninitialized. Ensure credentials (ADC or GEMINI_API_KEY) are configured."
		if onEvent != nil {
			onEvent(AgentEvent{
				Type: "text",
				Text: msg,
			})
		}
		return msg, nil
	}

	tr := otel.Tracer("sre-triage-agent")
	ctx, span := tr.Start(ctx, "agent.invoke",
		trace.WithAttributes(
			attribute.String("agent.context_id", contextID),
			attribute.String("agent.model", a.ModelName),
		),
	)
	defer span.End()

	slog.Default().InfoContext(ctx, "Starting SRE triage invocation", "context_id", contextID, "model", a.ModelName)

	systemContent := &genai.Content{
		Role: "system",
		Parts: []*genai.Part{
			{Text: GetSystemInstruction()},
		},
	}

	history := []*genai.Content{
		{
			Role: "user",
			Parts: []*genai.Part{
				{Text: userPrompt},
			},
		},
	}

	toolDefs := tools.GetToolDeclarations()
	maxTurns := 8

	type toolExecRecord struct {
		turn int
		name string
		args string
	}
	var executedTools []toolExecRecord

	for turn := 0; turn < maxTurns; turn++ {
		slog.Default().DebugContext(ctx, "Executing agent triage turn", "turn", turn, "context_id", contextID)

		genCfg := &genai.GenerateContentConfig{
			SystemInstruction: systemContent,
			Tools:             toolDefs,
		}

		genCtx, genSpan := tr.Start(ctx, "gemini.generate_content",
			trace.WithAttributes(
				attribute.String("genai.model", a.ModelName),
				attribute.Int("turn", turn),
			),
		)
		resp, err := a.client.Models.GenerateContent(genCtx, a.ModelName, history, genCfg)
		if err != nil {
			genSpan.RecordError(err)
			genSpan.SetStatus(codes.Error, err.Error())
			genSpan.End()
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			slog.Default().ErrorContext(ctx, "Gemini generate content failed", "turn", turn, "error", err)
			return "", fmt.Errorf("gemini generate content error: %w", err)
		}
		genSpan.End()

		if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
			slog.Default().WarnContext(ctx, "No response candidates returned by model", "turn", turn)
			msg := "No response generated by model."
			if onEvent != nil {
				onEvent(AgentEvent{
					Type: "text",
					Text: msg,
				})
			}
			return msg, nil
		}

		candidate := resp.Candidates[0]
		history = append(history, candidate.Content)

		var functionCalls []*genai.FunctionCall
		for _, part := range candidate.Content.Parts {
			if part.FunctionCall != nil {
				functionCalls = append(functionCalls, part.FunctionCall)
			}
		}

		// If no tools were called, return the text content
		if len(functionCalls) == 0 {
			var textOutput string
			for _, part := range candidate.Content.Parts {
				if part.Text != "" {
					textOutput += part.Text
				}
			}
			slog.Default().InfoContext(ctx, "Agent completed triage", "context_id", contextID, "turns", turn+1)

			if len(executedTools) > 0 {
				var traceSection strings.Builder
				traceSection.WriteString(fmt.Sprintf("\n\n---\n<details class=\"tool-trace\">\n<summary><b>🛠️ Tool Execution Trace (%d tool calls)</b></summary>\n\n", len(executedTools)))
				traceSection.WriteString("| Turn | Tool | Arguments |\n")
				traceSection.WriteString("|:---|:---|:---|\n")
				for _, rec := range executedTools {
					cleanArgs := rec.args
					if len(cleanArgs) > 80 {
						cleanArgs = cleanArgs[:77] + "..."
					}
					traceSection.WriteString(fmt.Sprintf("| Turn %d | `code`%s`/code` | `code`%s`/code` |\n", rec.turn, rec.name, cleanArgs))
				}
				traceSection.WriteString("\n</details>\n")
				textOutput += strings.ReplaceAll(traceSection.String(), "`code`", "<code>")
				textOutput = strings.ReplaceAll(textOutput, "`/code`", "</code>")
			}

			if onEvent != nil {
				onEvent(AgentEvent{
					Type: "text",
					Text: textOutput,
				})
			}

			return textOutput, nil
		}

		// Execute function calls
		var responseParts []*genai.Part
		for _, fc := range functionCalls {
			argsBytes, _ := json.Marshal(fc.Args)
			executedTools = append(executedTools, toolExecRecord{
				turn: turn + 1,
				name: fc.Name,
				args: string(argsBytes),
			})

			if onEvent != nil {
				onEvent(AgentEvent{
					Type: "functionCall",
					Name: fc.Name,
					Args: fc.Args,
				})
			}

			slog.Default().InfoContext(ctx, "Executing tool call", "tool", fc.Name, "context_id", contextID)
			toolCtx, toolSpan := tr.Start(ctx, "tool."+fc.Name,
				trace.WithAttributes(
					attribute.String("tool.name", fc.Name),
				),
			)

			result, err := a.dispatcher(fc.Name, fc.Args)
			if err != nil {
				toolSpan.RecordError(err)
				toolSpan.SetStatus(codes.Error, err.Error())
				slog.Default().WarnContext(toolCtx, "Tool execution returned error", "tool", fc.Name, "error", err)
				result = map[string]string{"error": err.Error()}
			}
			toolSpan.End()

			resultMap, ok := result.(map[string]any)
			if !ok {
				b, marshalErr := json.Marshal(result)
				if marshalErr == nil {
					var m map[string]any
					if unmarshalErr := json.Unmarshal(b, &m); unmarshalErr == nil {
						resultMap = m
						ok = true
					}
				}
				if !ok {
					resultMap = map[string]any{"output": result}
				}
			}

			if onEvent != nil {
				onEvent(AgentEvent{
					Type:     "functionResponse",
					Name:     fc.Name,
					Response: resultMap,
				})
			}

			responseParts = append(responseParts, &genai.Part{
				FunctionResponse: &genai.FunctionResponse{
					Name:     fc.Name,
					Response: resultMap,
				},
			})
		}

		history = append(history, &genai.Content{
			Role:  "tool",
			Parts: responseParts,
		})
	}

	slog.Default().WarnContext(ctx, "Reached maximum triage loop iterations", "context_id", contextID)
	msg := "Reached maximum triage loop iterations."
	if onEvent != nil {
		onEvent(AgentEvent{
			Type: "text",
			Text: msg,
		})
	}
	return msg, nil
}

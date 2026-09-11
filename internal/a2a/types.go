package a2a

import "encoding/json"

type AgentCard struct {
	ProtocolVersion     string            `json:"protocolVersion"`
	Name                string            `json:"name"`
	Description         string            `json:"description"`
	URL                 string            `json:"url"`
	PreferredTransport  string            `json:"preferredTransport"`
	SupportedInterfaces []AgentInterface  `json:"supportedInterfaces"`
	Capabilities        AgentCapabilities `json:"capabilities"`
	DefaultInputModes   []string          `json:"defaultInputModes"`
	DefaultOutputModes  []string          `json:"defaultOutputModes"`
	Skills              []AgentSkill      `json:"skills"`
}

type AgentInterface struct {
	URL             string `json:"url"`
	ProtocolBinding string `json:"protocolBinding"`
	ProtocolVersion string `json:"protocolVersion"`
}

type AgentCapabilities struct {
	Streaming bool `json:"streaming"`
}

type AgentSkill struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type JSONRPCRequest struct {
	JSONRPC string            `json:"jsonrpc"`
	ID      any               `json:"id"`
	Method  string            `json:"method"`
	Params  MessageSendParams `json:"params"`
}

type MessageSendParams struct {
	Message A2AMessage `json:"message"`
}

type A2AMessage struct {
	Role      string    `json:"role"`
	Parts     []A2APart `json:"parts"`
	TaskID    string    `json:"taskId,omitempty"`
	ContextID string    `json:"contextId,omitempty"`
}

func (m *A2AMessage) UnmarshalJSON(data []byte) error {
	type rawMessage struct {
		Role      string    `json:"role"`
		Parts     []A2APart `json:"parts"`
		TaskID    string    `json:"taskId,omitempty"`
		ContextID string    `json:"contextId,omitempty"`
	}
	var wrapped struct {
		Message *rawMessage `json:"message"`
	}
	if err := json.Unmarshal(data, &wrapped); err == nil && wrapped.Message != nil {
		m.Role = wrapped.Message.Role
		m.Parts = wrapped.Message.Parts
		m.TaskID = wrapped.Message.TaskID
		m.ContextID = wrapped.Message.ContextID
		if m.Role == "ROLE_AGENT" {
			m.Role = "agent"
		}
		return nil
	}

	var direct rawMessage
	if err := json.Unmarshal(data, &direct); err != nil {
		return err
	}
	m.Role = direct.Role
	m.Parts = direct.Parts
	m.TaskID = direct.TaskID
	m.ContextID = direct.ContextID
	if m.Role == "ROLE_AGENT" {
		m.Role = "agent"
	}
	return nil
}

func (m A2AMessage) MarshalJSON() ([]byte, error) {
	role := m.Role
	if role == "agent" || role == "" {
		role = "ROLE_AGENT"
	}
	type messagePayload struct {
		Role      string    `json:"role"`
		Parts     []A2APart `json:"parts"`
		TaskID    string    `json:"taskId,omitempty"`
		ContextID string    `json:"contextId,omitempty"`
	}
	wrapped := struct {
		Message messagePayload `json:"message"`
	}{
		Message: messagePayload{
			Role:      role,
			Parts:     m.Parts,
			TaskID:    m.TaskID,
			ContextID: m.ContextID,
		},
	}
	return json.Marshal(wrapped)
}

type A2APart struct {
	Kind string `json:"kind,omitempty"`
	Text string `json:"text,omitempty"`
	URL  string `json:"url,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      any           `json:"id"`
	Result  *A2AMessage   `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Место для работы с нейросетями

package llm

import (
	"encoding/json"
)

// ModelType представляет тип модели нейросети
type ModelType string

const (
	ModelTypeOpenAI ModelType = "openai"
	ModelTypeClaude ModelType = "claude"
	ModelTypeGrok   ModelType = "grok"
	ModelTypeGemini ModelType = "gemini"
)

// ModelTier представляет уровень модели
type ModelTier string

const (
	ModelTierBase    ModelTier = "base"    // Базовый уровень (для всех пользователей)
	ModelTierPremium ModelTier = "premium" // Премиум уровень (для подписчиков Premium и Pro)
	ModelTierPro     ModelTier = "pro"     // Профессиональный уровень (только для подписчиков Pro)
)

// Request представляет запрос к нейросети
type Request struct {
	UserID         int64                  `json:"user_id"`
	UserMessage    string                 `json:"user_message"`
	ModelType      ModelType              `json:"model_type"`
	ModelName      string                 `json:"model_name"`
	SystemPrompt   string                 `json:"system_prompt,omitempty"`
	ConversationID string                 `json:"conversation_id,omitempty"`
	MessageHistory []Message              `json:"message_history,omitempty"`
	MaxTokens      int                    `json:"max_tokens,omitempty"`
	Temperature    float64                `json:"temperature,omitempty"`
	Options        map[string]interface{} `json:"options,omitempty"`
}

// Response представляет ответ от нейросети
type Response struct {
	UserID           int64                  `json:"user_id"`
	RequestID        string                 `json:"request_id"`
	ModelType        ModelType              `json:"model_type"`
	ModelName        string                 `json:"model_name"`
	ResponseText     string                 `json:"response_text"`
	PromptTokens     int                    `json:"prompt_tokens"`
	CompletionTokens int                    `json:"completion_tokens"`
	TotalTokens      int                    `json:"total_tokens"`
	NeuronsCost      int                    `json:"neurons_cost"`
	Cached           bool                   `json:"cached"`
	Error            string                 `json:"error,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

// Message представляет сообщение в диалоге
type Message struct {
	Role    string `json:"role"`    // system, user, assistant
	Content string `json:"content"` // текст сообщения
}

// ModelConfig представляет конфигурацию модели
type ModelConfig struct {
	ID                string    `json:"id"`
	Type              ModelType `json:"type"`
	Name              string    `json:"name"`
	DisplayName       string    `json:"display_name"`
	Description       string    `json:"description"`
	Tier              ModelTier `json:"tier"`
	MaxTokensContext  int       `json:"max_tokens_context"`  // Максимальное количество токенов в контексте
	MaxTokensResponse int       `json:"max_tokens_response"` // Максимальное количество токенов в ответе
	NeuronsCost       int       `json:"neurons_cost"`        // Стоимость запроса в нейронах
	SupportedFeatures []string  `json:"supported_features"`  // Поддерживаемые функции
	Enabled           bool      `json:"enabled"`             // Доступна ли модель
}

// ModelInfoResponse возвращает информацию о доступных моделях
type ModelInfoResponse struct {
	Models []ModelConfig `json:"models"`
}

// ToJSON преобразует запрос в JSON
func (r *Request) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}

// FromJSON преобразует JSON в запрос
func (r *Request) FromJSON(data []byte) error {
	return json.Unmarshal(data, r)
}

// ToJSON преобразует ответ в JSON
func (r *Response) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}

// FromJSON преобразует JSON в ответ
func (r *Response) FromJSON(data []byte) error {
	return json.Unmarshal(data, r)
}

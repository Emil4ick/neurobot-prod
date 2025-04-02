package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"

	"neurobot-prod/internal/config"
)

// GrokClient представляет клиент для работы с API Grok
type GrokClient struct {
	config     config.GrokConfig
	httpClient *http.Client
	log        *zap.Logger
}

// GrokRequest представляет запрос к API Grok
type GrokRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	TopP        float64   `json:"top_p,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

// GrokResponse представляет ответ от API Grok
type GrokResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Usage   struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
		Index        int    `json:"index"`
	} `json:"choices"`
}

// NewGrokClient создает новый клиент Grok
func NewGrokClient(config config.GrokConfig, log *zap.Logger) *GrokClient {
	return &GrokClient{
		config: config,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		log: log.Named("grok_client"),
	}
}

// ProcessRequest обрабатывает запрос к нейросети Grok
func (c *GrokClient) ProcessRequest(ctx context.Context, request *Request) (*Response, error) {
	// Определяем модель для запроса
	modelName := c.getModelName(request.ModelName)

	// Создаем сообщения для запроса
	messages := make([]Message, 0)

	// Добавляем системный промпт, если он есть
	if request.SystemPrompt != "" {
		messages = append(messages, Message{
			Role:    "system",
			Content: request.SystemPrompt,
		})
	}

	// Добавляем историю сообщений, если она есть
	if len(request.MessageHistory) > 0 {
		messages = append(messages, request.MessageHistory...)
	}

	// Добавляем текущее сообщение пользователя
	messages = append(messages, Message{
		Role:    "user",
		Content: request.UserMessage,
	})

	// Создаем запрос к API Grok
	grokReq := GrokRequest{
		Model:    modelName,
		Messages: messages,
		Stream:   false,
	}

	// Устанавливаем максимальное количество токенов, если указано
	if request.MaxTokens > 0 {
		grokReq.MaxTokens = request.MaxTokens
	}

	// Устанавливаем температуру, если указана
	if request.Temperature > 0 {
		grokReq.Temperature = request.Temperature
	} else {
		grokReq.Temperature = 0.7 // Значение по умолчанию
	}

	// Создаем JSON для запроса
	jsonData, err := json.Marshal(grokReq)
	if err != nil {
		return nil, fmt.Errorf("ошибка сериализации запроса: %w", err)
	}

	// Создаем HTTP-запрос
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.groq.com/openai/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("ошибка создания HTTP-запроса: %w", err)
	}

	// Устанавливаем заголовки
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.ApiKey)

	// Отправляем запрос
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка отправки запроса: %w", err)
	}
	defer resp.Body.Close()

	// Проверяем код ответа
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ошибка API: код %d, тело: %s", resp.StatusCode, string(bodyBytes))
	}

	// Разбираем ответ
	var grokResp GrokResponse
	if err := json.NewDecoder(resp.Body).Decode(&grokResp); err != nil {
		return nil, fmt.Errorf("ошибка разбора ответа: %w", err)
	}

	// Проверяем, что есть хотя бы один выбор
	if len(grokResp.Choices) == 0 {
		return nil, fmt.Errorf("в ответе нет вариантов")
	}

	// Создаем ответ
	response := &Response{
		UserID:           request.UserID,
		RequestID:        grokResp.ID,
		ModelType:        ModelTypeGrok,
		ModelName:        modelName,
		ResponseText:     grokResp.Choices[0].Message.Content,
		PromptTokens:     grokResp.Usage.PromptTokens,
		CompletionTokens: grokResp.Usage.CompletionTokens,
		TotalTokens:      grokResp.Usage.TotalTokens,
		Metadata: map[string]interface{}{
			"finish_reason": grokResp.Choices[0].FinishReason,
		},
	}

	return response, nil
}

// getModelName возвращает имя модели для API
func (c *GrokClient) getModelName(requestedModel string) string {
	switch requestedModel {
	case "grok-2":
		return c.config.ProModel
	case "grok-1":
		return c.config.BaseModel
	default:
		// Если модель не распознана, используем базовую
		return c.config.BaseModel
	}
}

// GetModelInfo возвращает информацию о доступных моделях
func (c *GrokClient) GetModelInfo() []ModelConfig {
	return []ModelConfig{
		{
			ID:                "grok-1",
			Type:              ModelTypeGrok,
			Name:              "grok-1",
			DisplayName:       "Grok 1",
			Description:       "Базовая модель Grok с хорошим соотношением цены и качества",
			Tier:              ModelTierBase,
			MaxTokensContext:  4096,
			MaxTokensResponse: 2048,
			NeuronsCost:       1,
			SupportedFeatures: []string{"chat", "text-completion"},
			Enabled:           true,
		},
		{
			ID:                "grok-2",
			Type:              ModelTypeGrok,
			Name:              "grok-2",
			DisplayName:       "Grok 2",
			Description:       "Продвинутая модель с расширенными возможностями",
			Tier:              ModelTierPro,
			MaxTokensContext:  8192,
			MaxTokensResponse: 4096,
			NeuronsCost:       5,
			SupportedFeatures: []string{"chat", "text-completion", "reasoning"},
			Enabled:           true,
		},
	}
}

// Клиент OpenAI

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

// OpenAIClient представляет клиент для работы с API OpenAI
type OpenAIClient struct {
	config     config.OpenAIConfig
	httpClient *http.Client
	log        *zap.Logger
}

// OpenAIRequest представляет запрос к API OpenAI
type OpenAIRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	TopP        float64   `json:"top_p,omitempty"`
	N           int       `json:"n,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
	Stop        []string  `json:"stop,omitempty"`
	User        string    `json:"user,omitempty"`
}

// OpenAIResponse представляет ответ от API OpenAI
type OpenAIResponse struct {
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

// NewOpenAIClient создает новый клиент OpenAI
func NewOpenAIClient(config config.OpenAIConfig, log *zap.Logger) *OpenAIClient {
	return &OpenAIClient{
		config: config,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		log: log.Named("openai_client"),
	}
}

// ProcessRequest обрабатывает запрос к нейросети OpenAI
func (c *OpenAIClient) ProcessRequest(ctx context.Context, request *Request) (*Response, error) {
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

	// Создаем запрос к API OpenAI
	openaiReq := OpenAIRequest{
		Model:    modelName,
		Messages: messages,
	}

	// Устанавливаем максимальное количество токенов, если указано
	if request.MaxTokens > 0 {
		openaiReq.MaxTokens = request.MaxTokens
	}

	// Устанавливаем температуру, если указана
	if request.Temperature > 0 {
		openaiReq.Temperature = request.Temperature
	} else {
		openaiReq.Temperature = 0.7 // Значение по умолчанию
	}

	// Создаем JSON для запроса
	jsonData, err := json.Marshal(openaiReq)
	if err != nil {
		return nil, fmt.Errorf("ошибка сериализации запроса: %w", err)
	}

	// Создаем HTTP-запрос
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(jsonData))
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
	var openaiResp OpenAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&openaiResp); err != nil {
		return nil, fmt.Errorf("ошибка разбора ответа: %w", err)
	}

	// Проверяем, что есть хотя бы один выбор
	if len(openaiResp.Choices) == 0 {
		return nil, fmt.Errorf("в ответе нет вариантов")
	}

	// Создаем ответ
	response := &Response{
		UserID:           request.UserID,
		RequestID:        openaiResp.ID,
		ModelType:        ModelTypeOpenAI,
		ModelName:        modelName,
		ResponseText:     openaiResp.Choices[0].Message.Content,
		PromptTokens:     openaiResp.Usage.PromptTokens,
		CompletionTokens: openaiResp.Usage.CompletionTokens,
		TotalTokens:      openaiResp.Usage.TotalTokens,
		Metadata: map[string]interface{}{
			"finish_reason": openaiResp.Choices[0].FinishReason,
		},
	}

	return response, nil
}

// getModelName возвращает имя модели для API
func (c *OpenAIClient) getModelName(requestedModel string) string {
	switch requestedModel {
	case "gpt-4o":
		return c.config.ProModel
	case "gpt-4o-mini":
		return c.config.PremiumModel
	case "gpt-3.5-turbo":
		return c.config.BaseModel
	default:
		// Если модель не распознана, используем базовую
		return c.config.BaseModel
	}
}

// GetModelInfo возвращает информацию о доступных моделях
func (c *OpenAIClient) GetModelInfo() []ModelConfig {
	return []ModelConfig{
		{
			ID:                "gpt-3.5-turbo",
			Type:              ModelTypeOpenAI,
			Name:              "gpt-3.5-turbo",
			DisplayName:       "GPT-3.5 Turbo",
			Description:       "Быстрая и эффективная модель для общих задач",
			Tier:              ModelTierBase,
			MaxTokensContext:  4096,
			MaxTokensResponse: 2048,
			NeuronsCost:       1,
			SupportedFeatures: []string{"chat", "text-completion"},
			Enabled:           true,
		},
		{
			ID:                "gpt-4o-mini",
			Type:              ModelTypeOpenAI,
			Name:              "gpt-4o-mini",
			DisplayName:       "GPT-4o mini",
			Description:       "Улучшенная модель с расширенными возможностями",
			Tier:              ModelTierPremium,
			MaxTokensContext:  8192,
			MaxTokensResponse: 4096,
			NeuronsCost:       3,
			SupportedFeatures: []string{"chat", "text-completion", "code-generation"},
			Enabled:           true,
		},
		{
			ID:                "gpt-4o",
			Type:              ModelTypeOpenAI,
			Name:              "gpt-4o",
			DisplayName:       "GPT-4o",
			Description:       "Продвинутая модель с максимальными возможностями",
			Tier:              ModelTierPro,
			MaxTokensContext:  16384,
			MaxTokensResponse: 8192,
			NeuronsCost:       5,
			SupportedFeatures: []string{"chat", "text-completion", "code-generation", "reasoning"},
			Enabled:           true,
		},
	}
}

// Клиент Claude

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

// ClaudeClient представляет клиент для работы с API Claude
type ClaudeClient struct {
	config     config.ClaudeConfig
	httpClient *http.Client
	log        *zap.Logger
}

// ClaudeRequest представляет запрос к API Claude
type ClaudeRequest struct {
	Model       string          `json:"model"`
	Messages    []ClaudeMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
	System      string          `json:"system,omitempty"`
}

// ClaudeMessage представляет сообщение для API Claude
type ClaudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ClaudeResponse представляет ответ от API Claude
type ClaudeResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Model        string `json:"model"`
	StopReason   string `json:"stop_reason"`
	StopSequence string `json:"stop_sequence"`
	Usage        struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// NewClaudeClient создает новый клиент Claude
func NewClaudeClient(config config.ClaudeConfig, log *zap.Logger) *ClaudeClient {
	return &ClaudeClient{
		config: config,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		log: log.Named("claude_client"),
	}
}

// ProcessRequest обрабатывает запрос к нейросети Claude
func (c *ClaudeClient) ProcessRequest(ctx context.Context, request *Request) (*Response, error) {
	// Определяем модель для запроса
	modelName := c.getModelName(request.ModelName)

	// Создаем сообщения для запроса
	var claudeMessages []ClaudeMessage

	// Добавляем историю сообщений, если она есть
	if len(request.MessageHistory) > 0 {
		for _, msg := range request.MessageHistory {
			claudeRole := msg.Role
			if claudeRole == "assistant" {
				claudeRole = "assistant"
			} else if claudeRole == "system" {
				// Системные сообщения обрабатываются отдельно в Claude
				continue
			} else {
				claudeRole = "user"
			}

			claudeMessages = append(claudeMessages, ClaudeMessage{
				Role:    claudeRole,
				Content: msg.Content,
			})
		}
	}

	// Добавляем текущее сообщение пользователя
	claudeMessages = append(claudeMessages, ClaudeMessage{
		Role:    "user",
		Content: request.UserMessage,
	})

	// Создаем запрос к API Claude
	claudeReq := ClaudeRequest{
		Model:    modelName,
		Messages: claudeMessages,
	}

	// Устанавливаем системный промпт, если он есть
	if request.SystemPrompt != "" {
		claudeReq.System = request.SystemPrompt
	}

	// Устанавливаем максимальное количество токенов, если указано
	if request.MaxTokens > 0 {
		claudeReq.MaxTokens = request.MaxTokens
	} else {
		claudeReq.MaxTokens = 2048 // Значение по умолчанию
	}

	// Устанавливаем температуру, если указана
	if request.Temperature > 0 {
		claudeReq.Temperature = request.Temperature
	} else {
		claudeReq.Temperature = 0.7 // Значение по умолчанию
	}

	// Создаем JSON для запроса
	jsonData, err := json.Marshal(claudeReq)
	if err != nil {
		return nil, fmt.Errorf("ошибка сериализации запроса: %w", err)
	}

	// Создаем HTTP-запрос
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("ошибка создания HTTP-запроса: %w", err)
	}

	// Устанавливаем заголовки
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.config.ApiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

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
	var claudeResp ClaudeResponse
	if err := json.NewDecoder(resp.Body).Decode(&claudeResp); err != nil {
		return nil, fmt.Errorf("ошибка разбора ответа: %w", err)
	}

	// Извлекаем текст из ответа
	var responseText string
	for _, content := range claudeResp.Content {
		if content.Type == "text" {
			responseText += content.Text
		}
	}

	// Создаем ответ
	response := &Response{
		UserID:           request.UserID,
		RequestID:        claudeResp.ID,
		ModelType:        ModelTypeClaude,
		ModelName:        modelName,
		ResponseText:     responseText,
		PromptTokens:     claudeResp.Usage.InputTokens,
		CompletionTokens: claudeResp.Usage.OutputTokens,
		TotalTokens:      claudeResp.Usage.InputTokens + claudeResp.Usage.OutputTokens,
		Metadata: map[string]interface{}{
			"stop_reason":   claudeResp.StopReason,
			"stop_sequence": claudeResp.StopSequence,
		},
	}

	return response, nil
}

// getModelName возвращает имя модели для API
func (c *ClaudeClient) getModelName(requestedModel string) string {
	switch requestedModel {
	case "claude-3-opus":
		return c.config.ProModel
	case "claude-3-sonnet":
		return c.config.PremiumModel
	case "claude-3-haiku":
		return c.config.BaseModel
	default:
		// Если модель не распознана, используем базовую
		return c.config.BaseModel
	}
}

// GetModelInfo возвращает информацию о доступных моделях
func (c *ClaudeClient) GetModelInfo() []ModelConfig {
	return []ModelConfig{
		{
			ID:                "claude-3-haiku",
			Type:              ModelTypeClaude,
			Name:              "claude-3-haiku",
			DisplayName:       "Claude 3 Haiku",
			Description:       "Быстрая и эффективная модель для повседневных задач",
			Tier:              ModelTierBase,
			MaxTokensContext:  4096,
			MaxTokensResponse: 2048,
			NeuronsCost:       1,
			SupportedFeatures: []string{"chat", "text-completion"},
			Enabled:           true,
		},
		{
			ID:                "claude-3-sonnet",
			Type:              ModelTypeClaude,
			Name:              "claude-3-sonnet",
			DisplayName:       "Claude 3 Sonnet",
			Description:       "Сбалансированная модель для сложных задач",
			Tier:              ModelTierPremium,
			MaxTokensContext:  8192,
			MaxTokensResponse: 4096,
			NeuronsCost:       3,
			SupportedFeatures: []string{"chat", "text-completion", "reasoning"},
			Enabled:           true,
		},
		{
			ID:                "claude-3-opus",
			Type:              ModelTypeClaude,
			Name:              "claude-3-opus",
			DisplayName:       "Claude 3 Opus",
			Description:       "Самая мощная модель Claude с максимальными возможностями",
			Tier:              ModelTierPro,
			MaxTokensContext:  16384,
			MaxTokensResponse: 8192,
			NeuronsCost:       5,
			SupportedFeatures: []string{"chat", "text-completion", "reasoning", "code-generation"},
			Enabled:           true,
		},
	}
}

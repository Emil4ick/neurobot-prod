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

// GeminiClient представляет клиент для работы с API Gemini
type GeminiClient struct {
	config     config.GeminiConfig
	httpClient *http.Client
	log        *zap.Logger
}

// GeminiRequest представляет запрос к API Gemini
type GeminiRequest struct {
	Contents         []GeminiContent        `json:"contents"`
	GenerationConfig GeminiGenerationConfig `json:"generationConfig,omitempty"`
	SafetySettings   []GeminiSafetySetting  `json:"safetySettings,omitempty"`
}

// GeminiContent представляет содержимое запроса к Gemini
type GeminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []GeminiPart `json:"parts"`
}

// GeminiPart представляет часть содержимого запроса к Gemini
type GeminiPart struct {
	Text string `json:"text"`
}

// GeminiGenerationConfig представляет настройки генерации для Gemini
type GeminiGenerationConfig struct {
	Temperature     float64 `json:"temperature,omitempty"`
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
	TopP            float64 `json:"topP,omitempty"`
	TopK            int     `json:"topK,omitempty"`
}

// GeminiSafetySetting представляет настройки безопасности для Gemini
type GeminiSafetySetting struct {
	Category  string `json:"category"`
	Threshold string `json:"threshold"`
}

// GeminiResponse представляет ответ от API Gemini
type GeminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	PromptFeedback struct {
		BlockReason string `json:"blockReason,omitempty"`
	} `json:"promptFeedback"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
}

// NewGeminiClient создает новый клиент Gemini
func NewGeminiClient(config config.GeminiConfig, log *zap.Logger) *GeminiClient {
	return &GeminiClient{
		config: config,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		log: log.Named("gemini_client"),
	}
}

// ProcessRequest обрабатывает запрос к нейросети Gemini
func (c *GeminiClient) ProcessRequest(ctx context.Context, request *Request) (*Response, error) {
	// Определяем модель для запроса
	modelName := c.getModelName(request.ModelName)

	// Создаем содержимое для запроса
	var contents []GeminiContent

	// Обрабатываем системный промпт, если он есть
	if request.SystemPrompt != "" {
		contents = append(contents, GeminiContent{
			Role: "user",
			Parts: []GeminiPart{
				{Text: "Системный контекст: " + request.SystemPrompt},
			},
		})
	}

	// Добавляем историю сообщений, если она есть
	if len(request.MessageHistory) > 0 {
		for _, msg := range request.MessageHistory {
			role := "user"
			if msg.Role == "assistant" {
				role = "model"
			}

			contents = append(contents, GeminiContent{
				Role: role,
				Parts: []GeminiPart{
					{Text: msg.Content},
				},
			})
		}
	}

	// Добавляем текущее сообщение пользователя
	contents = append(contents, GeminiContent{
		Role: "user",
		Parts: []GeminiPart{
			{Text: request.UserMessage},
		},
	})

	// Создаем запрос к API Gemini
	geminiReq := GeminiRequest{
		Contents: contents,
		GenerationConfig: GeminiGenerationConfig{
			Temperature:     request.Temperature,
			MaxOutputTokens: request.MaxTokens,
		},
		// Устанавливаем стандартные настройки безопасности
		SafetySettings: []GeminiSafetySetting{
			{Category: "HARM_CATEGORY_HARASSMENT", Threshold: "BLOCK_MEDIUM_AND_ABOVE"},
			{Category: "HARM_CATEGORY_HATE_SPEECH", Threshold: "BLOCK_MEDIUM_AND_ABOVE"},
			{Category: "HARM_CATEGORY_SEXUALLY_EXPLICIT", Threshold: "BLOCK_MEDIUM_AND_ABOVE"},
			{Category: "HARM_CATEGORY_DANGEROUS_CONTENT", Threshold: "BLOCK_MEDIUM_AND_ABOVE"},
		},
	}

	// Если температура не указана, устанавливаем по умолчанию
	if geminiReq.GenerationConfig.Temperature == 0 {
		geminiReq.GenerationConfig.Temperature = 0.7
	}

	// Если максимальное количество токенов не указано, устанавливаем по умолчанию
	if geminiReq.GenerationConfig.MaxOutputTokens == 0 {
		geminiReq.GenerationConfig.MaxOutputTokens = 2048
	}

	// Создаем JSON для запроса
	jsonData, err := json.Marshal(geminiReq)
	if err != nil {
		return nil, fmt.Errorf("ошибка сериализации запроса: %w", err)
	}

	// Формируем URL для запроса
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		modelName, c.config.ApiKey)

	// Создаем HTTP-запрос
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("ошибка создания HTTP-запроса: %w", err)
	}

	// Устанавливаем заголовки
	req.Header.Set("Content-Type", "application/json")

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
	var geminiResp GeminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		return nil, fmt.Errorf("ошибка разбора ответа: %w", err)
	}

	// Проверяем, что есть результаты
	if len(geminiResp.Candidates) == 0 {
		return nil, fmt.Errorf("нет результатов в ответе")
	}

	// Получаем текст ответа
	var responseText string
	for _, part := range geminiResp.Candidates[0].Content.Parts {
		responseText += part.Text
	}

	// Создаем ответ
	response := &Response{
		UserID:           request.UserID,
		RequestID:        "gemini-" + time.Now().Format("20060102150405"),
		ModelType:        ModelTypeGemini,
		ModelName:        modelName,
		ResponseText:     responseText,
		PromptTokens:     geminiResp.UsageMetadata.PromptTokenCount,
		CompletionTokens: geminiResp.UsageMetadata.CandidatesTokenCount,
		TotalTokens:      geminiResp.UsageMetadata.TotalTokenCount,
		Metadata: map[string]interface{}{
			"finish_reason": geminiResp.Candidates[0].FinishReason,
		},
	}

	return response, nil
}

// getModelName возвращает имя модели для API
func (c *GeminiClient) getModelName(requestedModel string) string {
	switch requestedModel {
	case "gemini-1.5-pro":
		return c.config.PremiumModel
	case "gemini-1.0-pro":
		return c.config.BaseModel
	default:
		// Если модель не распознана, используем базовую
		return c.config.BaseModel
	}
}

// GetModelInfo возвращает информацию о доступных моделях
func (c *GeminiClient) GetModelInfo() []ModelConfig {
	return []ModelConfig{
		{
			ID:                "gemini-1.0-pro",
			Type:              ModelTypeGemini,
			Name:              "gemini-1.0-pro",
			DisplayName:       "Gemini 1.0 Pro",
			Description:       "Универсальная модель для общих задач",
			Tier:              ModelTierBase,
			MaxTokensContext:  4096,
			MaxTokensResponse: 2048,
			NeuronsCost:       1,
			SupportedFeatures: []string{"chat", "text-completion"},
			Enabled:           true,
		},
		{
			ID:                "gemini-1.5-pro",
			Type:              ModelTypeGemini,
			Name:              "gemini-1.5-pro",
			DisplayName:       "Gemini 1.5 Pro",
			Description:       "Продвинутая модель с улучшенными возможностями",
			Tier:              ModelTierPremium,
			MaxTokensContext:  8192,
			MaxTokensResponse: 4096,
			NeuronsCost:       3,
			SupportedFeatures: []string{"chat", "text-completion", "reasoning"},
			Enabled:           true,
		},
	}
}

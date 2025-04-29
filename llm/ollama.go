package llm

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OllamaConfig stores configuration for the Ollama LLM
type OllamaConfig struct {
	Enabled     bool   `mapstructure:"enabled"`
	URL         string `mapstructure:"url"`
	Model       string `mapstructure:"model"`
	MaxArticles int    `mapstructure:"max_articles"`
	Timeout     int    `mapstructure:"timeout"`
}

// DefaultOllamaConfig returns default config
func DefaultOllamaConfig() OllamaConfig {
	return OllamaConfig{
		Enabled:     true,
		URL:         "http://localhost:11434",
		Model:       "qwen3:32b",
		MaxArticles: 100,
		Timeout:     3000,
	}
}

// OllamaRequest is the request structure for Ollama API
type OllamaRequest struct {
	Model   string                 `json:"model"`
	Prompt  string                 `json:"prompt"`
	Stream  bool                   `json:"stream"`
	Options map[string]interface{} `json:"options,omitempty"`
}

// OllamaResponse is the response structure from Ollama API
type OllamaResponse struct {
	Model         string `json:"model"`
	Response      string `json:"response"`
	Done          bool   `json:"done"`
	Context       []int  `json:"context,omitempty"`
	TotalDuration int64  `json:"total_duration,omitempty"`
	Error         string `json:"error,omitempty"`
}

// StreamingResponseMsg represents a streaming response chunk from Ollama
type StreamingResponseMsg struct {
	Content string
	Done    bool
	Err     error
}

// AskOllama sends an arbitrary prompt to the Ollama LLM and returns the response
func AskOllama(prompt string, config OllamaConfig) (string, error) {
	if !config.Enabled {
		return "", fmt.Errorf("LLM is disabled in config")
	}

	// Create the request
	reqBody := OllamaRequest{
		Model:  config.Model,
		Prompt: prompt,
		Stream: false,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("error marshaling request: %w", err)
	}

	client := &http.Client{
		Timeout: time.Duration(config.Timeout) * time.Second,
	}

	apiURL := strings.TrimSuffix(config.URL, "/") + "/api/generate"
	resp, err := client.Post(apiURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("error calling Ollama API at %s: %w", apiURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Ollama API returned non-200 status code %d: %s", resp.StatusCode, string(body))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %w", err)
	}

	var ollamaResp OllamaResponse
	if err := json.Unmarshal(bodyBytes, &ollamaResp); err != nil {
		return "", fmt.Errorf("error parsing Ollama response: %w", err)
	}

	if ollamaResp.Error != "" {
		return "", fmt.Errorf("Ollama API error: %s", ollamaResp.Error)
	}

	return ollamaResp.Response, nil
}

// AskOllamaStreaming sends a prompt to Ollama and streams back responses
// It returns a channel that will receive streaming responses
func AskOllamaStreaming(prompt string, config OllamaConfig) chan StreamingResponseMsg {
	responseChannel := make(chan StreamingResponseMsg, 10)

	go func() {
		defer close(responseChannel)

		if !config.Enabled {
			responseChannel <- StreamingResponseMsg{Content: "", Done: true, Err: fmt.Errorf("LLM is disabled in config")}
			return
		}

		// Create the streaming request
		reqBody := OllamaRequest{
			Model:  config.Model,
			Prompt: prompt,
			Stream: true, // Enable streaming
		}

		jsonData, err := json.Marshal(reqBody)
		if err != nil {
			responseChannel <- StreamingResponseMsg{Content: "", Done: true, Err: fmt.Errorf("error marshaling request: %w", err)}
			return
		}

		client := &http.Client{
			Timeout: time.Duration(config.Timeout) * time.Second,
		}

		apiURL := strings.TrimSuffix(config.URL, "/") + "/api/generate"
		resp, err := client.Post(apiURL, "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			responseChannel <- StreamingResponseMsg{Content: "", Done: true, Err: fmt.Errorf("error calling Ollama API at %s: %w", apiURL, err)}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			responseChannel <- StreamingResponseMsg{Content: "", Done: true, Err: fmt.Errorf("Ollama API returned non-200 status code %d: %s", resp.StatusCode, string(body))}
			return
		}

		// 使用更大的缓冲区以处理长响应行
		scanner := bufio.NewScanner(resp.Body)
		buf := make([]byte, 0, 64*1024) // 默认缓冲区大小是 64KB
		scanner.Buffer(buf, 1024*1024)  // 设置最大缓冲区为 1MB

		// 累积大约 5 个字符后发送更新
		buffer := ""
		bufferThreshold := 1

		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}

			var streamResp OllamaResponse
			if err := json.Unmarshal([]byte(line), &streamResp); err != nil {
				responseChannel <- StreamingResponseMsg{Content: "", Done: true, Err: fmt.Errorf("error parsing streaming response: %w", err)}
				return
			}

			// Check for errors in response
			if streamResp.Error != "" {
				responseChannel <- StreamingResponseMsg{Content: "", Done: true, Err: fmt.Errorf("Ollama API error: %s", streamResp.Error)}
				return
			}

			// Add to buffer
			buffer += streamResp.Response

			// Send updates in chunks to avoid too many small updates
			if len(buffer) >= bufferThreshold || streamResp.Done {
				responseChannel <- StreamingResponseMsg{Content: buffer, Done: streamResp.Done, Err: nil}
				buffer = ""
			}

			if streamResp.Done {
				break
			}
		}

		if err := scanner.Err(); err != nil {
			responseChannel <- StreamingResponseMsg{Content: "", Done: true, Err: fmt.Errorf("error reading stream: %w", err)}
		}
	}()

	return responseChannel
}

// GetAvailableModels retrieves the list of available models from Ollama
func GetAvailableModels(config OllamaConfig) ([]string, error) {
	client := &http.Client{
		Timeout: time.Duration(config.Timeout) * time.Second,
	}

	resp, err := client.Get(config.URL + "/api/tags")
	if err != nil {
		return nil, fmt.Errorf("error fetching models from Ollama: %w", err)
	}
	defer resp.Body.Close()

	type Model struct {
		Name string `json:"name"`
	}

	type ModelsResponse struct {
		Models []Model `json:"models"`
	}

	var modelsResp ModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("error parsing models response: %w", err)
	}

	var modelNames []string
	for _, model := range modelsResp.Models {
		modelNames = append(modelNames, model.Name)
	}

	return modelNames, nil
}

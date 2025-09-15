package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"time"
)

// InputPayload defines the new input object
type InputPayload struct {
	Language string `json:"language"`
	Title    string `json:"title"`
	Captions string `json:"captions"`
}

// RequestPayload defines the expected input parameters
type RequestPayload struct {
	Prompt         string       `json:"prompt"`
	Model          string       `json:"model"`
	Output         string       `json:"output"`
	Input          InputPayload `json:"input"`
	PromptTemplate string       `json:"prompt_template"`
}

// ResponsePayload defines the response structure
type ResponsePayload struct {
	Prompt          string       `json:"prompt"`
	Model           string       `json:"model"`
	Output          string       `json:"output"`
	Input           InputPayload `json:"input"`
	Result          interface{}  `json:"result,omitempty"`
	Error           string       `json:"error,omitempty"`
	RequestDuration string       `json:"request_duration"`
}

// loadPromptTemplate loads both system and user prompt files
// Returns: systemPrompt, userPrompt, error
func loadPromptTemplate(templateName string) (string, string, error) {
	if templateName == "" {
		return "", "", nil
	}

	systemPath := filepath.Join("/app/prompts", templateName+".system.prompt.txt")
	userPath := filepath.Join("/app/prompts", templateName+".user.prompt.txt")

	systemContent, err := ioutil.ReadFile(systemPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to load system prompt '%s': %w", systemPath, err)
	}

	userContent, err := ioutil.ReadFile(userPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to load user prompt '%s': %w", userPath, err)
	}

	return string(systemContent), string(userContent), nil
}

func summarizeHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RequestPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	// Load template if provided
	systemPrompt, userPrompt, err := loadPromptTemplate(req.PromptTemplate)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req.Prompt = systemPrompt + " - " + userPrompt

	// Print current prompt for debugging
	fmt.Println("System Prompt:", systemPrompt)
	fmt.Println("User Prompt:", userPrompt)

	resp := ResponsePayload{
		Prompt: req.Prompt,
		Model:  req.Model,
		Output: req.Output,
		Input:  req.Input,
	}

	switch req.Model {
	case "deepseekr1":
		// Format userPrompt with inputs
		title := req.Input.Title
		lang := req.Input.Language
		caption := req.Input.Captions
		userPromptFormatted := fmt.Sprintf(userPrompt, title, lang, caption)

		summary, err := CallDeepSeek(systemPrompt, userPromptFormatted)
		if err != nil {
			resp.Error = err.Error()
		} else {
			resp.Result = summary
		}

	case "gemini-2.0-flash":
		// Build Gemini prompt
		userPromptFormatted := fmt.Sprintf(userPrompt, req.Input.Title, req.Input.Language, req.Input.Captions)

		summary, err := CallGemini(systemPrompt, userPromptFormatted)
		if err != nil {
			resp.Error = err.Error()
		} else {
			resp.Result = summary
		}

	default:
		resp.Result = map[string]interface{}{
			"echo":  req.Prompt,
			"input": req.Input,
		}
	}

	duration := time.Since(start)
	resp.RequestDuration = duration.String()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func main() {
	http.HandleFunc("/summarize", summarizeHandler)
	log.Println("Server running on http://localhost:3030")
	log.Fatal(http.ListenAndServe(":3030", nil))
}

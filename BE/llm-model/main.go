package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// RequestPayload defines the expected input parameters
type RequestPayload struct {
	Prompt string `json:"prompt"`
	Model  string `json:"model"`
	Output string `json:"output"`
}

// ResponsePayload defines the response structure
type ResponsePayload struct {
	Prompt          string      `json:"prompt"`
	Model           string      `json:"model"`
	Output          string      `json:"output"`
	Result          interface{} `json:"result,omitempty"`
	Error           string      `json:"error,omitempty"`
	RequestDuration string      `json:"request_duration"` // new field
}

func summarizeHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now() // capture start time

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RequestPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	resp := ResponsePayload{
		Prompt: req.Prompt,
		Model:  req.Model,
		Output: req.Output,
	}

	if req.Model == "deepseekr1" {
		summary, err := CallDeepSeek(req.Prompt)
		if err != nil {
			resp.Error = err.Error()
		} else {
			resp.Result = summary
		}
	} else {
		resp.Result = map[string]string{"echo": req.Prompt}
	}

	// compute request duration
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

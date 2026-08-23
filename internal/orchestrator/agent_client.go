package orchestrator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// callAgentService POSTs a JSON payload to the given path on the Python
// agent service and unmarshals the JSON response into result.
// result must be a pointer (e.g. &ExtractedFacts{}).
func callAgentService(path string, payload interface{}, result interface{}) error {
	baseURL := os.Getenv("AGENT_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8000"
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := http.Post(baseURL+path, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to call agent service %s: %w", path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response from %s: %w", path, err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("agent service %s returned %d: %s", path, resp.StatusCode, string(respBody))
	}

	if err := json.Unmarshal(respBody, result); err != nil {
		return fmt.Errorf("failed to parse response from %s: %w", path, err)
	}

	return nil
}
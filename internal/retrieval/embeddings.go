package retrieval

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// EmbedText calls Gemini's embedding endpoint (text-embedding-004),
// returning a 768-dimensional vector.
func EmbedText(text string) ([]float32, error) {
	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GOOGLE_API_KEY not set")
	}

	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/gemini-embedding-001:embedContent?key=%s",
		apiKey,
	)

	reqBody := map[string]interface{}{
		"model": "models/gemini-embedding-001",
		"content": map[string]interface{}{
			"parts": []map[string]string{{"text": text}},
		},
		"outputDimensionality": 768,
	}
	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini embeddings error (%d): %s", resp.StatusCode, string(respBody))
	}

	var parsed struct {
		Embedding struct {
			Values []float32 `json:"values"`
		} `json:"embedding"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, err
	}
	if len(parsed.Embedding.Values) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}

	return parsed.Embedding.Values, nil
}

// ChunkPolicyText splits policy text into paragraph-based chunks capped
// near maxChunkChars, so no single chunk is too large to embed cleanly.
func ChunkPolicyText(text string, maxChunkChars int) []string {
	paragraphs := strings.Split(text, "\n\n")
	var chunks []string
	var current strings.Builder

	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if current.Len()+len(p) > maxChunkChars && current.Len() > 0 {
			chunks = append(chunks, current.String())
			current.Reset()
		}
		current.WriteString(p)
		current.WriteString("\n\n")
	}
	if current.Len() > 0 {
		chunks = append(chunks, current.String())
	}
	return chunks
}
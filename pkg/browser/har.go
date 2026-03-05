package browser

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

type HarRecorder struct {
	mu      sync.Mutex
	entries []HarEntry
	active  bool
	started time.Time
}

type HarEntry struct {
	StartedDateTime string     `json:"startedDateTime"`
	Time            float64    `json:"time"`
	Request         HarRequest `json:"request"`
	Response        HarResponse `json:"response"`
}

type HarRequest struct {
	Method      string         `json:"method"`
	URL         string         `json:"url"`
	HTTPVersion string         `json:"httpVersion"`
	Headers     []HarHeader    `json:"headers"`
	QueryString []HarQueryParam `json:"queryString"`
	BodySize    int            `json:"bodySize"`
}

type HarResponse struct {
	Status      int                `json:"status"`
	StatusText  string             `json:"statusText"`
	HTTPVersion string             `json:"httpVersion"`
	Headers     []HarHeader        `json:"headers"`
	Content     *HarResponseContent `json:"content,omitempty"`
	BodySize    int                `json:"bodySize"`
}

type HarResponseContent struct {
	Size     int    `json:"size"`
	MimeType string `json:"mimeType"`
	Text     string `json:"text"`
}

type HarHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type HarQueryParam struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func (h *HarRecorder) Start() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.active = true
	h.started = time.Now()
	h.entries = nil
}

func (h *HarRecorder) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.active = false
}

func (h *HarRecorder) IsActive() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.active
}

func (h *HarRecorder) AddEntry(req *Request, resp *Response) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.active {
		return
	}

	reqHeaders := make([]HarHeader, 0, len(req.Headers))
	for k, v := range req.Headers {
		reqHeaders = append(reqHeaders, HarHeader{Name: k, Value: v})
	}

	respHeaders := make([]HarHeader, 0)
	status := 0
	statusText := ""
	if resp != nil {
		for k, v := range resp.Headers {
			respHeaders = append(respHeaders, HarHeader{Name: k, Value: v})
		}
		status = resp.Status
		statusText = resp.statusText
	}

	h.entries = append(h.entries, HarEntry{
		StartedDateTime: time.Now().Format(time.RFC3339Nano),
		Request: HarRequest{
			Method:      req.Method,
			URL:         req.URL,
			HTTPVersion: "HTTP/1.1",
			Headers:     reqHeaders,
			BodySize:    len(req.PostData),
		},
		Response: HarResponse{
			Status:      status,
			StatusText:  statusText,
			HTTPVersion: "HTTP/1.1",
			Headers:     respHeaders,
		},
	})
}

func (h *HarRecorder) SaveAs(path string) error {
	h.mu.Lock()
	entries := make([]HarEntry, len(h.entries))
	copy(entries, h.entries)
	h.mu.Unlock()

	har := map[string]any{
		"log": map[string]any{
			"version": "1.2",
			"creator": map[string]string{
				"name":    "gowright",
				"version": "1.0",
			},
			"entries": entries,
		},
	}

	data, err := json.MarshalIndent(har, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (h *HarRecorder) Entries() []HarEntry {
	h.mu.Lock()
	defer h.mu.Unlock()
	entries := make([]HarEntry, len(h.entries))
	copy(entries, h.entries)
	return entries
}

func LoadHar(path string) ([]HarEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var har struct {
		Log struct {
			Entries []HarEntry `json:"entries"`
		} `json:"log"`
	}
	if err := json.Unmarshal(data, &har); err != nil {
		return nil, err
	}
	return har.Log.Entries, nil
}

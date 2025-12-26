package mcp

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestQuantClient_WithMeta_RequestBodyContainsMeta(t *testing.T) {
	mockHTTP := NewMockHTTPClient()
	mockLogger := NewMockLogger()

	// Capture request body
	var capturedBody []byte
	mockHTTP.ResponseFunc = func(req *http.Request) (*http.Response, error) {
		// Read and capture request body
		if req.Body != nil {
			bodyBytes, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("failed to read request body: %v", err)
			}
			capturedBody = bodyBytes
			// Restore body for potential retries
			req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`{"choices":[{"message":{"content":"test response"}}]}`)),
		}, nil
	}

	// Create QuantClient with mock HTTP client
	client := NewQuantClientWithOptions(
		WithHTTPClient(mockHTTP.ToHTTPClient()),
		WithLogger(mockLogger),
		WithAPIKey("test-key"),
		WithBaseURL("https://api.test.com"),
	)

	// Cast to QuantClient to access WithMeta
	quantClient, ok := client.(*QuantClient)
	if !ok {
		t.Fatal("client should be *QuantClient")
	}

	// Set meta using WithMeta
	clientWithMeta := quantClient.WithMeta("test_key", "test_value")
	// Cast back to QuantClient to chain WithMeta calls
	clientWithMeta = clientWithMeta.(*QuantClient).WithMeta("another_key", 123)

	// Make API call
	_, err := clientWithMeta.CallWithMessages("system prompt", "user prompt")
	if err != nil {
		t.Fatalf("should not error: %v", err)
	}

	// Verify request was made
	requests := mockHTTP.GetRequests()
	if len(requests) == 0 {
		t.Fatal("expected at least one request")
	}

	// Parse captured request body
	var body map[string]interface{}
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("failed to unmarshal request body: %v", err)
	}

	// Verify meta field exists
	meta, exists := body["meta"]
	if !exists {
		t.Error("request body should contain 'meta' field")
		t.Logf("Request body: %s", string(capturedBody))
		return
	}

	// Verify meta is a map
	metaMap, ok := meta.(map[string]interface{})
	if !ok {
		t.Errorf("meta should be a map, got %T", meta)
		return
	}

	// Verify meta contains the set values
	if metaMap["test_key"] != "test_value" {
		t.Errorf("expected meta['test_key'] to be 'test_value', got %v", metaMap["test_key"])
	}

	if metaMap["another_key"] != float64(123) { // JSON numbers are decoded as float64
		t.Errorf("expected meta['another_key'] to be 123, got %v", metaMap["another_key"])
	}
}

func TestQuantClient_WithMeta_MultipleCalls(t *testing.T) {
	mockHTTP := NewMockHTTPClient()
	mockLogger := NewMockLogger()

	// Capture request body
	var capturedBody []byte
	mockHTTP.ResponseFunc = func(req *http.Request) (*http.Response, error) {
		if req.Body != nil {
			bodyBytes, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("failed to read request body: %v", err)
			}
			capturedBody = bodyBytes
			req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`{"choices":[{"message":{"content":"test response"}}]}`)),
		}, nil
	}

	client := NewQuantClientWithOptions(
		WithHTTPClient(mockHTTP.ToHTTPClient()),
		WithLogger(mockLogger),
		WithAPIKey("test-key"),
		WithBaseURL("https://api.test.com"),
	)

	quantClient := client.(*QuantClient)

	// Chain multiple WithMeta calls
	clientWithMeta := quantClient.WithMeta("key1", "value1")
	clientWithMeta = clientWithMeta.(*QuantClient).WithMeta("key2", "value2")
	clientWithMeta = clientWithMeta.(*QuantClient).WithMeta("key3", "value3")

	_, err := clientWithMeta.CallWithMessages("system", "user")
	if err != nil {
		t.Fatalf("should not error: %v", err)
	}

	// Parse request body
	var body map[string]interface{}
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("failed to unmarshal request body: %v", err)
	}

	// Verify all meta values are present
	meta, exists := body["meta"]
	if !exists {
		t.Fatal("request body should contain 'meta' field")
	}

	metaMap, ok := meta.(map[string]interface{})
	if !ok {
		t.Fatalf("meta should be a map, got %T", meta)
	}

	expectedValues := map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}

	for key, expectedValue := range expectedValues {
		if metaMap[key] != expectedValue {
			t.Errorf("expected meta['%s'] to be '%v', got %v", key, expectedValue, metaMap[key])
		}
	}
}

func TestQuantClient_WithMeta_EmptyMeta(t *testing.T) {
	mockHTTP := NewMockHTTPClient()
	mockLogger := NewMockLogger()

	var capturedBody []byte
	mockHTTP.ResponseFunc = func(req *http.Request) (*http.Response, error) {
		if req.Body != nil {
			bodyBytes, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("failed to read request body: %v", err)
			}
			capturedBody = bodyBytes
			req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`{"choices":[{"message":{"content":"test response"}}]}`)),
		}, nil
	}

	client := NewQuantClientWithOptions(
		WithHTTPClient(mockHTTP.ToHTTPClient()),
		WithLogger(mockLogger),
		WithAPIKey("test-key"),
		WithBaseURL("https://api.test.com"),
	)

	// Call without WithMeta - meta should be empty map or nil
	_, err := client.CallWithMessages("system", "user")
	if err != nil {
		t.Fatalf("should not error: %v", err)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("failed to unmarshal request body: %v", err)
	}

	// Verify meta field exists (even if empty)
	meta, exists := body["meta"]
	if !exists {
		t.Error("request body should contain 'meta' field even when empty")
		return
	}

	// Meta should be empty map or nil when not set
	if meta != nil {
		metaMap, ok := meta.(map[string]interface{})
		if ok && len(metaMap) != 0 {
			t.Errorf("expected empty meta when WithMeta not called, got %v", metaMap)
		}
	}
}

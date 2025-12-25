package mcp

import (
	"net/http"
)

const (
	ProviderQuant = "quant"
)

type QuantClient struct {
	*Client
}

// NewQuantClient creates Quant client (backward compatible)
//
// Deprecated: Recommend using NewQuantClientWithOptions for better flexibility
func NewQuantClient() AIClient {
	return NewQuantClientWithOptions()
}

// NewQuantClientWithOptions creates Quant client (supports options pattern)
//
// Usage examples:
//
//	// Basic usage
//	client := mcp.NewQuantClientWithOptions()
//
//	// Custom configuration
//	client := mcp.NewQuantClientWithOptions(
//	    mcp.WithAPIKey("sk-xxx"),
//	    mcp.WithLogger(customLogger),
//	    mcp.WithTimeout(60*time.Second),
//	)
func NewQuantClientWithOptions(opts ...ClientOption) AIClient {
	// 1. Create DeepSeek preset options
	quantOpts := []ClientOption{
		WithProvider(ProviderQuant),
	}

	// 2. Merge user options (user options have higher priority)
	allOpts := append(quantOpts, opts...)

	// 3. Create base client
	baseClient := NewClient(allOpts...).(*Client)

	// 4. Create Quant client
	quantClient := &QuantClient{
		Client: baseClient,
	}

	// 5. Set hooks to point to DeepSeekClient (implement dynamic dispatch)
	baseClient.hooks = quantClient

	return quantClient
}

func (quantClient *QuantClient) PromptFormat() PromptFormat {
	return PromptFormatJSON
}

func (quantClient *QuantClient) SetAPIKey(apiKey string, customURL string, customModel string) {
	quantClient.APIKey = apiKey

	if len(apiKey) > 8 {
		quantClient.logger.Infof("🔧 [MCP] Quant API Key: %s...%s", apiKey[:4], apiKey[len(apiKey)-4:])
	}
	if customURL != "" {
		quantClient.BaseURL = customURL
		quantClient.logger.Infof("🔧 [MCP] Quant using custom BaseURL: %s", customURL)
	} else {
		quantClient.logger.Infof("🔧 [MCP] Quant using default BaseURL: %s", quantClient.BaseURL)
	}
	if customModel != "" {
		quantClient.Model = customModel
		quantClient.logger.Infof("🔧 [MCP] Quant using custom Model: %s", customModel)
	} else {
		quantClient.logger.Infof("🔧 [MCP] Quant using default Model: %s", quantClient.Model)
	}
}

func (quantClient *QuantClient) setAuthHeader(reqHeaders http.Header) {
	quantClient.Client.setAuthHeader(reqHeaders)
}

package mcpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JieWaZi/NeuxAlly/server/constants"
	"github.com/JieWaZi/NeuxAlly/server/repositories/models"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestStreamableHTTPListsToolsWithConfiguredHeaders(t *testing.T) {
	t.Parallel()

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "mcpclient-test",
		Version: "1",
	}, nil)
	mcp.AddTool[map[string]any, any](server, &mcp.Tool{
		Name:        "search",
		Description: "搜索代码",
	}, func(
		context.Context,
		*mcp.CallToolRequest,
		map[string]any,
	) (*mcp.CallToolResult, any, error) {
		return nil, nil, nil
	})
	handler := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		nil,
	)
	httpServer := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		if request.Header.Get("Authorization") != "Bearer secret" {
			http.Error(writer, "missing authorization", http.StatusUnauthorized)
			return
		}
		handler.ServeHTTP(writer, request)
	}))
	t.Cleanup(httpServer.Close)

	result := Test(context.Background(), models.MCPConnector{
		Transport: constants.MCPTransportStreamableHTTP,
		URL:       httpServer.URL,
		TimeoutMS: 5_000,
		Headers: []models.MCPSecretDefinition{{
			Name:       "Authorization",
			Configured: true,
			Value:      "Bearer secret",
		}},
	})

	if result.Status != constants.MCPTestStatusSucceeded {
		t.Fatalf("test status = %q, error = %q", result.Status, result.Error)
	}
	if len(result.Tools) != 1 ||
		result.Tools[0].Name != "search" ||
		result.Tools[0].Description != "搜索代码" {
		t.Fatalf("tools = %+v", result.Tools)
	}
}

func TestConfiguredSecretRequiresDecryptedValue(t *testing.T) {
	t.Parallel()

	result := Test(context.Background(), models.MCPConnector{
		Transport: constants.MCPTransportStreamableHTTP,
		URL:       "https://example.invalid/mcp",
		TimeoutMS: 100,
		Headers: []models.MCPSecretDefinition{{
			Name:       "Authorization",
			Configured: true,
		}},
	})

	if result.Status != constants.MCPTestStatusFailed {
		t.Fatalf("test status = %q, want failed", result.Status)
	}
	if result.Error == "" {
		t.Fatal("test error is empty")
	}
}

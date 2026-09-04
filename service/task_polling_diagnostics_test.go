package service

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskDiagnosticFromResponseAcceptsOnlyFixedInternalContract(t *testing.T) {
	response := &http.Response{Header: make(http.Header)}
	response.Header.Set("X-Haomao-Diagnostic-Code", "upstream_rate_limited")
	response.Header.Set("X-Haomao-Diagnostic-Stage", "upstream_poll")
	response.Header.Set("X-Haomao-Upstream-Status", "429")

	diagnostic := taskDiagnosticFromResponse(response, []byte(`{"provider":"must-not-be-stored"}`))
	require.NotNil(t, diagnostic)
	assert.Equal(t, "upstream_rate_limited", diagnostic.Code)
	assert.Equal(t, "upstream_poll", diagnostic.Stage)
	assert.Equal(t, http.StatusTooManyRequests, diagnostic.UpstreamHTTPStatus)
	assert.True(t, diagnostic.Retryable)

	response.Header.Set("X-Haomao-Diagnostic-Code", "private-provider-name")
	assert.Nil(t, taskDiagnosticFromResponse(response, nil))
}

func TestTaskDiagnosticFromResponseUsesSanitizedBodyCode(t *testing.T) {
	response := &http.Response{Header: make(http.Header)}
	diagnostic := taskDiagnosticFromResponse(response, []byte(`{"error":{"code":"upstream_balance_insufficient"}}`))
	require.NotNil(t, diagnostic)
	assert.Equal(t, "upstream_balance_insufficient", diagnostic.Code)
	assert.Empty(t, diagnostic.UpstreamHTTPStatus)
}

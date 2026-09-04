package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func runTurnstileRequest(t *testing.T, path, secret string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST(path, TurnstileCheck(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodPost, path, nil)
	if secret != "" {
		request.Header.Set(consoleLoginSecretHeader, secret)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func TestTurnstileCheckAllowsTrustedConsoleLoginOnly(t *testing.T) {
	previousEnabled := common.TurnstileCheckEnabled
	common.TurnstileCheckEnabled = true
	t.Cleanup(func() { common.TurnstileCheckEnabled = previousEnabled })

	const secret = "console-login-secret-with-more-than-32-characters"
	t.Setenv("CONSOLE_LOGIN_SHARED_SECRET", secret)

	require.Equal(t, http.StatusNoContent, runTurnstileRequest(t, "/api/user/login", secret).Code)
	require.Equal(t, http.StatusOK, runTurnstileRequest(t, "/api/user/login", "wrong-secret").Code)
	require.Equal(t, http.StatusOK, runTurnstileRequest(t, "/api/user/register", secret).Code)
}

func TestTurnstileCheckRejectsShortOrMissingServerSecret(t *testing.T) {
	previousEnabled := common.TurnstileCheckEnabled
	common.TurnstileCheckEnabled = true
	t.Cleanup(func() { common.TurnstileCheckEnabled = previousEnabled })

	t.Setenv("CONSOLE_LOGIN_SHARED_SECRET", "too-short")
	require.Equal(t, http.StatusOK, runTurnstileRequest(t, "/api/user/login", "too-short").Code)

	t.Setenv("CONSOLE_LOGIN_SHARED_SECRET", "")
	require.Equal(t, http.StatusOK, runTurnstileRequest(t, "/api/user/login", "").Code)
}

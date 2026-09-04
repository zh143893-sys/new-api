package middleware

import (
	"crypto/subtle"
	"net/http"
	"net/url"
	"os"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

type turnstileCheckResponse struct {
	Success bool `json:"success"`
}

const consoleLoginSecretHeader = "X-Console-Login-Secret"

func trustedConsoleLogin(c *gin.Context) bool {
	if c.Request.Method != http.MethodPost || c.Request.URL.Path != "/api/user/login" {
		return false
	}
	serverSecret := os.Getenv("CONSOLE_LOGIN_SHARED_SECRET")
	if len(serverSecret) < 32 {
		return false
	}
	requestSecret := c.GetHeader(consoleLoginSecretHeader)
	return subtle.ConstantTimeCompare([]byte(requestSecret), []byte(serverSecret)) == 1
}

func TurnstileCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		if common.TurnstileCheckEnabled && !trustedConsoleLogin(c) {
			response := c.Query("turnstile")
			if response == "" {
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"message": "Turnstile token 为空",
				})
				c.Abort()
				return
			}
			rawRes, err := http.PostForm("https://challenges.cloudflare.com/turnstile/v0/siteverify", url.Values{
				"secret":   {common.TurnstileSecretKey},
				"response": {response},
				"remoteip": {c.ClientIP()},
			})
			if err != nil {
				common.SysLog(err.Error())
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"message": err.Error(),
				})
				c.Abort()
				return
			}
			defer rawRes.Body.Close()
			var res turnstileCheckResponse
			err = common.DecodeJson(rawRes.Body, &res)
			if err != nil {
				common.SysLog(err.Error())
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"message": err.Error(),
				})
				c.Abort()
				return
			}
			if !res.Success {
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"message": "Turnstile 校验失败，请刷新重试！",
				})
				c.Abort()
				return
			}
		}
		c.Next()
	}
}

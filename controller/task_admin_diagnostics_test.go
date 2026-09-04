package controller

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTasksToDtoSeparatesAdminDiagnosticsFromUserResponse(t *testing.T) {
	task := &model.Task{
		TaskID:     "task_public123",
		ChannelId:  51,
		Status:     model.TaskStatusFailure,
		FailReason: "provider secret detail",
		Properties: model.Properties{OriginModelName: "haomao-pro", UpstreamModelName: "private-model"},
		PrivateData: model.TaskPrivateData{Diagnostic: &model.TaskDiagnostic{
			Code: "upstream_balance_insufficient", Stage: "upstream_poll",
			UpstreamHTTPStatus: http.StatusPaymentRequired, RecordedAt: 123,
		}},
	}
	task.SetData(map[string]any{"provider": "private-provider", "url": "https://private.invalid/video"})

	admin := tasksToDto([]*model.Task{task}, true)[0]
	require.NotNil(t, admin.AdminDiagnostic)
	assert.Equal(t, "UPSTREAM_BALANCE_INSUFFICIENT", admin.AdminDiagnostic.Code)
	assert.Equal(t, "上游线路余额不足", admin.AdminDiagnostic.Summary)
	assert.Equal(t, http.StatusPaymentRequired, admin.AdminDiagnostic.UpstreamHTTPStatus)

	user := tasksToDto([]*model.Task{task}, false)[0]
	assert.Nil(t, user.AdminDiagnostic)
	assert.Equal(t, 0, user.ChannelId)
	assert.Empty(t, user.Data)
	assert.Empty(t, user.ResultURL)
	assert.Equal(t, "视频生成失败", user.FailReason)
	assert.Equal(t, map[string]string{"origin_model_name": "haomao-pro"}, user.Properties)
}

func TestServeTaskPreviewSupportsRangeWithSignedShortLivedURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	t.Setenv("VIDEO_DELIVERY_CACHE_DIR", root)
	oldSecret := common.SessionSecret
	common.SessionSecret = "test-preview-secret"
	t.Cleanup(func() { common.SessionSecret = oldSecret })

	taskID := "task_preview123"
	path, _ := taskPreviewFile(taskID)
	require.NoError(t, os.WriteFile(path, []byte("0123456789"), 0o600))
	expires := time.Now().Add(time.Minute).Unix()
	signature := common.GenerateHMACWithKey([]byte(common.SessionSecret), fmt.Sprintf("%s:%d", taskID, expires))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "task_id", Value: taskID}}
	ctx.Request = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/task/preview/%s?expires=%d&signature=%s", taskID, expires, signature), nil)
	ctx.Request.Header.Set("Range", "bytes=2-5")
	ServeTaskPreview(ctx)

	assert.Equal(t, http.StatusPartialContent, recorder.Code)
	assert.Equal(t, "2345", recorder.Body.String())
	assert.Equal(t, "private, no-store", recorder.Header().Get("Cache-Control"))
	assert.Equal(t, "video/mp4", recorder.Header().Get("Content-Type"))
	assert.Equal(t, filepath.Base(path), fmt.Sprintf("%x.mp4", sha256ForTest(taskID)))
}

func sha256ForTest(value string) [32]byte {
	return sha256.Sum256([]byte(value))
}

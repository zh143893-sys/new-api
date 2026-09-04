package controller

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func GetAllTask(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)

	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	// 解析其他查询参数
	queryParams := model.SyncTaskQueryParams{
		Platform:       constant.TaskPlatform(c.Query("platform")),
		TaskID:         c.Query("task_id"),
		Status:         c.Query("status"),
		Action:         c.Query("action"),
		StartTimestamp: startTimestamp,
		EndTimestamp:   endTimestamp,
		ChannelID:      c.Query("channel_id"),
	}

	items := model.TaskGetAllTasks(pageInfo.GetStartIdx(), pageInfo.GetPageSize(), queryParams)
	total := model.TaskCountAllTasks(queryParams)
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(tasksToDto(items, true))
	common.ApiSuccess(c, pageInfo)
}

func GetUserTask(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)

	userId := c.GetInt("id")

	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)

	queryParams := model.SyncTaskQueryParams{
		Platform:       constant.TaskPlatform(c.Query("platform")),
		TaskID:         c.Query("task_id"),
		Status:         c.Query("status"),
		Action:         c.Query("action"),
		StartTimestamp: startTimestamp,
		EndTimestamp:   endTimestamp,
	}

	items := model.TaskGetAllUserTask(userId, pageInfo.GetStartIdx(), pageInfo.GetPageSize(), queryParams)
	total := model.TaskCountAllUserTask(userId, queryParams)
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(tasksToDto(items, false))
	common.ApiSuccess(c, pageInfo)
}

func tasksToDto(tasks []*model.Task, fillUser bool) []*dto.TaskDto {
	var userIdMap map[int]*model.UserBase
	if fillUser {
		userIdMap = make(map[int]*model.UserBase)
		userIds := types.NewSet[int]()
		for _, task := range tasks {
			userIds.Add(task.UserId)
		}
		for _, userId := range userIds.Items() {
			cacheUser, err := model.GetUserCache(userId)
			if err == nil {
				userIdMap[userId] = cacheUser
			}
		}
	}
	result := make([]*dto.TaskDto, len(tasks))
	for i, task := range tasks {
		if fillUser {
			if user, ok := userIdMap[task.UserId]; ok {
				task.Username = user.Username
			}
		}
		item := relay.TaskModel2Dto(task)
		if fillUser {
			item.AdminDiagnostic = taskAdminDiagnostic(task)
			_, item.PreviewAvailable = taskPreviewFile(task.TaskID)
		} else if isHaomaoTask(task) {
			item.ChannelId = 0
			item.ResultURL = ""
			item.Data = nil
			item.Properties = map[string]string{"origin_model_name": task.Properties.OriginModelName}
			if task.Status == model.TaskStatusFailure {
				item.FailReason = "视频生成失败"
			}
		}
		result[i] = item
	}
	return result
}

func isHaomaoTask(task *model.Task) bool {
	switch task.Properties.OriginModelName {
	case "haomao-mini", "haomao-mini-no-water", "haomao-pro", "haomao-pro-10s", "haomao-plus", "haomao-max", "Luna":
		return true
	}
	return false
}

func taskAdminDiagnostic(task *model.Task) *dto.TaskAdminDiagnostic {
	code, stage, status, retryable, recordedAt := "", "", 0, false, int64(0)
	if task.PrivateData.Diagnostic != nil {
		diagnostic := task.PrivateData.Diagnostic
		code, stage = diagnostic.Code, diagnostic.Stage
		status, retryable, recordedAt = diagnostic.UpstreamHTTPStatus, diagnostic.Retryable, diagnostic.RecordedAt
	}
	historical := code == ""
	if code == "" {
		var data map[string]any
		if common.Unmarshal(task.Data, &data) == nil {
			if value, ok := data["phase"].(string); ok {
				stage = value
			}
			if value, ok := data["error"].(map[string]any); ok {
				code, _ = value["code"].(string)
			}
		}
	}
	if code == "" && strings.Contains(task.FailReason, "超时") {
		code, stage, retryable = "upstream_queue_timeout", "polling", true
	}
	if code == "" && task.Status == model.TaskStatusFailure {
		code, stage = "generation_failed", "upstream_generation"
	}
	if code == "" {
		return nil
	}

	summary, action := diagnosticText(code)
	if stage == "" {
		stage = "upstream_generation"
	}
	return &dto.TaskAdminDiagnostic{
		Code: strings.ToUpper(code), Stage: stage, Summary: summary, Action: action,
		UpstreamHTTPStatus: status, Retryable: retryable, RecordedAt: recordedAt, Historical: historical,
	}
}

func diagnosticText(code string) (string, string) {
	switch strings.ToLower(code) {
	case "upstream_balance_insufficient":
		return "上游线路余额不足", "充值或暂时关闭该线路"
	case "upstream_auth_failed":
		return "上游线路认证失败", "检查并轮换该线路密钥"
	case "upstream_rate_limited", "upstream_busy":
		return "上游线路触发限流或队列繁忙", "降低该线路并发或等待后重试"
	case "upstream_rejected":
		return "上游拒绝了请求参数或参考素材", "检查本任务规格和参考图"
	case "upstream_transport_error", "upstream_poll_unavailable":
		return "上游连接或任务查询暂时失败", "检查线路健康并继续查询原任务"
	case "upstream_unavailable":
		return "上游线路不可用；历史记录未保存更细状态", "检查该线路余额、认证和服务健康"
	case "upstream_queue_timeout":
		return "任务超过系统轮询等待上限", "先核查上游原任务，禁止重复提交"
	case "submission_unconfirmed":
		return "任务提交结果无法确认", "按幂等记录核对，禁止重复提交"
	case "asset_unavailable":
		return "成片已生成但交付缓存不可用", "恢复原成片后重新执行交付"
	case "delivery_review_required":
		return "成片交付校验未通过", "检查重封装和媒体校验记录"
	default:
		return "上游明确返回视频生成失败", "无需重新提交；确认退款已到账"
	}
}

func taskPreviewFile(taskID string) (string, bool) {
	root := strings.TrimSpace(os.Getenv("VIDEO_DELIVERY_CACHE_DIR"))
	if root == "" || !validPreviewTaskID(taskID) {
		return "", false
	}
	digest := sha256.Sum256([]byte(taskID))
	path := filepath.Join(root, fmt.Sprintf("%x.mp4", digest[:]))
	info, err := os.Lstat(path)
	return path, err == nil && info.Mode().IsRegular()
}

func validPreviewTaskID(taskID string) bool {
	if len(taskID) < 6 || len(taskID) > 191 || !strings.HasPrefix(taskID, "task_") {
		return false
	}
	for _, char := range taskID {
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') && char != '_' && char != '-' {
			return false
		}
	}
	return true
}

func MintTaskPreview(c *gin.Context) {
	taskID := c.Param("task_id")
	task, exists, err := model.GetByPublicTaskId(taskID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !exists || task.Status != model.TaskStatusSuccess || !isHaomaoTask(task) {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "没有可预览的成片"})
		return
	}
	if _, ready := taskPreviewFile(taskID); !ready {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "成片缓存已过期或尚未准备完成"})
		return
	}
	expires := time.Now().Add(5 * time.Minute).Unix()
	payload := fmt.Sprintf("%s:%d", taskID, expires)
	signature := common.GenerateHMACWithKey([]byte(common.SessionSecret), payload)
	common.ApiSuccess(c, gin.H{"url": fmt.Sprintf("/api/task/preview/%s?expires=%d&signature=%s", url.PathEscape(taskID), expires, signature)})
}

func ServeTaskPreview(c *gin.Context) {
	taskID := c.Param("task_id")
	expires, err := strconv.ParseInt(c.Query("expires"), 10, 64)
	if err != nil || expires < time.Now().Unix() || expires > time.Now().Add(6*time.Minute).Unix() {
		c.Status(http.StatusForbidden)
		return
	}
	payload := fmt.Sprintf("%s:%d", taskID, expires)
	expected := common.GenerateHMACWithKey([]byte(common.SessionSecret), payload)
	if !hmac.Equal([]byte(c.Query("signature")), []byte(expected)) {
		c.Status(http.StatusForbidden)
		return
	}
	path, ready := taskPreviewFile(taskID)
	if !ready {
		c.Status(http.StatusNotFound)
		return
	}
	file, err := os.Open(path)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		c.Status(http.StatusNotFound)
		return
	}
	c.Header("Content-Type", "video/mp4")
	c.Header("Content-Disposition", "inline")
	c.Header("Cache-Control", "private, no-store")
	http.ServeContent(c.Writer, c.Request, taskID+".mp4", info.ModTime(), file)
}

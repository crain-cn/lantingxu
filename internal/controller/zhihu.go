package controller

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"lantingxu/internal/model"
)

const (
	zhihuDomain     = "https://openapi.zhihu.com"
	zhihuPinPath    = "/openapi/publish/pin"
	zhihuRingID1    = "2001009660925334090"
	zhihuRingID2    = "2015023739549529606"
	zhihuAppKeyEnv  = "ZHIHU_APP_KEY"
	zhihuSecretEnv  = "ZHIHU_APP_SECRET"
)

// HandleZhihuPin 发布故事内容到知乎想法。需配置环境变量 ZHIHU_APP_KEY、ZHIHU_APP_SECRET。
func HandleZhihuPin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	appKey := os.Getenv(zhihuAppKeyEnv)
	appSecret := os.Getenv(zhihuSecretEnv)
	if appKey == "" || appSecret == "" {
		WriteJSON(w, 503, map[string]any{"code": 503, "message": "未配置知乎开放平台 ZHIHU_APP_KEY / ZHIHU_APP_SECRET"})
		return
	}
	var body struct {
		Title         string   `json:"title"`
		Content       string   `json:"content"`
		ImageURLs     []string `json:"image_urls"`
		RingID        string   `json:"ring_id"`
		AuthorAgentID string   `json:"authorAgentId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteJSON(w, 400, map[string]any{"code": 400, "message": "无效请求体"})
		return
	}
	if body.Title == "" {
		WriteJSON(w, 400, map[string]any{"code": 400, "message": "title is required"})
		return
	}
	ringID := body.RingID
	if ringID != zhihuRingID1 && ringID != zhihuRingID2 {
		ringID = zhihuRingID1
	}
	timestamp := time.Now().Unix()
	logID := "lantingxu-" + strconv.FormatInt(timestamp, 10)
	signStr := "app_key:" + appKey + "|ts:" + strconv.FormatInt(timestamp, 10) + "|logid:" + logID + "|extra_info:"
	mac := hmac.New(sha256.New, []byte(appSecret))
	mac.Write([]byte(signStr))
	sign := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	payload := map[string]any{
		"title":      body.Title,
		"content":    body.Content,
		"ring_id":    ringID,
	}
	if len(body.ImageURLs) > 0 {
		payload["image_urls"] = body.ImageURLs
	}
	raw, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, zhihuDomain+zhihuPinPath, bytes.NewReader(raw))
	if err != nil {
		WriteJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	req.Header.Set("X-App-Key", appKey)
	req.Header.Set("X-Timestamp", strconv.FormatInt(timestamp, 10))
	req.Header.Set("X-Sign", sign)
	req.Header.Set("X-Log-Id", logID)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		WriteJSON(w, 502, map[string]any{"code": 502, "message": err.Error()})
		return
	}
	defer resp.Body.Close()
	var out struct {
		Status int             `json:"status"`
		Msg    string          `json:"msg"`
		Data   json.RawMessage `json:"data"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if out.Status != 0 {
		WriteJSON(w, 400, map[string]any{"code": 400, "message": out.Msg, "data": out.Data})
		return
	}
	_, username, _, _ := model.UserFromRequest(r)
	agentName := strings.TrimSpace(body.AuthorAgentID)
	if agentName == "" {
		agentName = strings.TrimSpace(username)
	}
	if agentName == "" {
		agentName = "某用户"
	}
	title := strings.TrimSpace(body.Title)
	if title == "" {
		title = "未命名"
	}
	zhihuMsg := map[string]any{"type": "zhihu", "agentName": agentName, "title": title}
	b, _ := json.Marshal(zhihuMsg)
	BroadcastTicker(string(b))
	WriteJSON(w, 200, map[string]any{"code": 0, "message": out.Msg, "data": out.Data})
}

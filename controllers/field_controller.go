package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
)

// FieldController 分野管理に関するコントローラー
type FieldController struct{}

// NewFieldController コントローラーのインスタンスを作成
func NewFieldController() *FieldController {
	return &FieldController{}
}

// ShowFields 分野管理ページを表示
func (fc *FieldController) ShowFields(c echo.Context) error {
	sess, err := session.Get("login-session", c)
	if err != nil {
		return c.Redirect(http.StatusSeeOther, "/")
	}

	// セッションの検証
	auth, ok := sess.Values["authenticated"].(bool)
	if !ok || !auth {
		return c.Redirect(http.StatusSeeOther, "/")
	}

	return c.File("views/admin.html")
}

// GetFields ルームIDに紐づくフィールド一覧を取得
func (fc *FieldController) GetFields(c echo.Context) error {
	sess, err := session.Get("login-session", c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"message": "セッションが無効です",
		})
	}

	// セッションの検証
	auth, ok := sess.Values["authenticated"].(bool)
	if !ok || !auth {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"message": "認証が必要です",
		})
	}

	roomID, ok := sess.Values["room_id"].(string)
	if !ok {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"message": "room_idの取得に失敗しました",
		})
	}

	supabaseURL := os.Getenv("SUPABASE_URL")
	supabaseKey := os.Getenv("SUPABASE_KEY")

	// フィールド一覧を取得するクエリ（room_id, field_name, priorityを取得）
	fullURL := fmt.Sprintf("%s/rest/v1/field?select=room_id,field_name,priority&room_id=eq.%s", supabaseURL, roomID)

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"message": "リクエストの作成に失敗しました",
		})
	}

	req.Header.Set("apikey", supabaseKey)
	req.Header.Set("Authorization", "Bearer "+supabaseKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"message": "APIリクエストに失敗しました",
		})
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"message": "レスポンスの読み取りに失敗しました",
		})
	}

	var fields []struct {
		RoomID    string `json:"room_id"`
		FieldName string `json:"field_name"`
		Priority  int    `json:"priority"`
	}
	if err := json.Unmarshal(body, &fields); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"message": "JSONのパースに失敗しました",
		})
	}

	// フロントエンド用のデータ構造に変換
	var responseFields []map[string]interface{}
	for _, field := range fields {
		responseFields = append(responseFields, map[string]interface{}{
			"id":       field.RoomID,
			"name":     field.FieldName,
			"priority": field.Priority,
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"fields": responseFields,
	})
}

// DeleteFields 選択された分野を削除
func (fc *FieldController) DeleteFields(c echo.Context) error {
	sess, err := session.Get("login-session", c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"message": "セッションが無効です",
		})
	}

	// セッションの検証
	auth, ok := sess.Values["authenticated"].(bool)
	if !ok || !auth {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"message": "認証が必要です",
		})
	}

	roomID, ok := sess.Values["room_id"].(string)
	if !ok {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"message": "room_idの取得に失敗しました",
		})
	}

	// リクエストボディから削除する分野名のリストを取得
	var requestBody struct {
		FieldNames []string `json:"field_names"`
	}
	if err := c.Bind(&requestBody); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"message": "リクエストの解析に失敗しました",
		})
	}

	if len(requestBody.FieldNames) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"message": "削除する分野が指定されていません",
		})
	}

	supabaseURL := os.Getenv("SUPABASE_URL")
	supabaseKey := os.Getenv("SUPABASE_KEY")

	// 削除に成功した数をカウント
	successCount := 0

	// 各分野を削除
	for _, fieldName := range requestBody.FieldNames {
		// URLエンコードを適用
		encodedRoomID := url.QueryEscape(roomID)
		encodedFieldName := url.QueryEscape(fieldName)

		// 削除クエリを構築
		fullURL := fmt.Sprintf("%s/rest/v1/field?room_id=eq.%s&field_name=eq.%s",
			supabaseURL, encodedRoomID, encodedFieldName)

		fmt.Printf("削除リクエスト - URL: %s\n", fullURL)

		req, err := http.NewRequest("DELETE", fullURL, nil)
		if err != nil {
			fmt.Printf("リクエスト作成エラー: %v\n", err)
			continue
		}

		req.Header.Set("apikey", supabaseKey)
		req.Header.Set("Authorization", "Bearer "+supabaseKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Prefer", "return=minimal")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Printf("削除リクエストエラー: %v\n", err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK {
			successCount++
			fmt.Printf("フィールド %s の削除に成功\n", fieldName)
		} else {
			bodyBytes, _ := io.ReadAll(resp.Body)
			fmt.Printf("削除エラー - Status: %d, Body: %s\n", resp.StatusCode, string(bodyBytes))
		}
	}

	if successCount == 0 {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"message": "分野の削除に失敗しました",
		})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": fmt.Sprintf("%d個の分野を削除しました", successCount),
	})
}

// AddField 新しい分野を追加
func (fc *FieldController) AddField(c echo.Context) error {
	sess, err := session.Get("login-session", c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"message": "セッションが無効です",
		})
	}

	// セッションの検証
	auth, ok := sess.Values["authenticated"].(bool)
	if !ok || !auth {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"message": "認証が必要です",
		})
	}

	roomID, ok := sess.Values["room_id"].(string)
	if !ok {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"message": "room_idの取得に失敗しました",
		})
	}

	// リクエストボディから分野名を取得
	var requestBody struct {
		FieldName string `json:"field_name"`
	}
	if err := c.Bind(&requestBody); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"message": "リクエストの解析に失敗しました",
		})
	}

	if requestBody.FieldName == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"message": "分野名が指定されていません",
		})
	}

	// 分野名を分割（カンマまたは読点で区切る）
	fieldNames := strings.FieldsFunc(requestBody.FieldName, func(r rune) bool {
		return r == ',' || r == '、'
	})

	// 空白を除去し、空の要素を削除
	var validFieldNames []string
	for _, name := range fieldNames {
		trimmed := strings.TrimSpace(name)
		if trimmed != "" {
			validFieldNames = append(validFieldNames, trimmed)
		}
	}

	if len(validFieldNames) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"message": "有効な分野名が指定されていません",
		})
	}

	supabaseURL := os.Getenv("SUPABASE_URL")
	supabaseKey := os.Getenv("SUPABASE_KEY")

	// 各分野名を追加
	addedCount := 0
	for _, fieldName := range validFieldNames {
		// 新しい分野を追加
		fullURL := supabaseURL + "/rest/v1/field"
		requestData := map[string]interface{}{
			"room_id":    roomID,
			"field_name": fieldName,
			"priority":   3, // デフォルトの興味の強さを3（普通）に設定
		}

		jsonData, err := json.Marshal(requestData)
		if err != nil {
			continue
		}

		req, err := http.NewRequest("POST", fullURL, bytes.NewBuffer(jsonData))
		if err != nil {
			continue
		}

		req.Header.Set("apikey", supabaseKey)
		req.Header.Set("Authorization", "Bearer "+supabaseKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Prefer", "return=minimal")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			continue
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusCreated {
			addedCount++
		}
	}

	if addedCount == 0 {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"message": "分野の追加に失敗しました",
		})
	}

	message := fmt.Sprintf("%d個の分野を追加しました", addedCount)
	return c.JSON(http.StatusOK, map[string]string{
		"message": message,
	})
}

// UpdateFieldPriority 分野の優先度を更新
func (fc *FieldController) UpdateFieldPriority(c echo.Context) error {
	sess, err := session.Get("login-session", c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"message": "セッションが無効です",
		})
	}

	// セッションの検証
	auth, ok := sess.Values["authenticated"].(bool)
	if !ok || !auth {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"message": "認証が必要です",
		})
	}

	roomID, ok := sess.Values["room_id"].(string)
	if !ok {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"message": "room_idの取得に失敗しました",
		})
	}

	// リクエストボディを解析
	var req struct {
		FieldName string `json:"field_name"`
		Priority  int    `json:"priority"`
	}
	if err := c.Bind(&req); err != nil {
		fmt.Printf("リクエストのバインドエラー: %v\n", err)
		return c.JSON(http.StatusBadRequest, map[string]string{
			"message": "無効なリクエストです",
		})
	}

	fmt.Printf("更新リクエスト - Room ID: %s, Field Name: %s, Priority: %d\n", roomID, req.FieldName, req.Priority)

	// 優先度の値を検証
	if req.Priority < 1 || req.Priority > 5 {
		fmt.Printf("無効な優先度: %d\n", req.Priority)
		return c.JSON(http.StatusBadRequest, map[string]string{
			"message": "優先度は1から5の間で指定してください",
		})
	}

	supabaseURL := os.Getenv("SUPABASE_URL")
	supabaseKey := os.Getenv("SUPABASE_KEY")

	// 優先度を更新
	encodedRoomID := url.QueryEscape(roomID)
	encodedFieldName := url.QueryEscape(req.FieldName)
	updateURL := fmt.Sprintf("%s/rest/v1/field?room_id=eq.%s&field_name=eq.%s",
		supabaseURL, encodedRoomID, encodedFieldName)
	fmt.Printf("更新URL: %s\n", updateURL)

	updateData := map[string]interface{}{
		"priority": req.Priority,
	}
	jsonData, err := json.Marshal(updateData)
	if err != nil {
		fmt.Printf("JSONの生成エラー: %v\n", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"message": "JSONの生成に失敗しました",
		})
	}

	updateReq, err := http.NewRequest("PATCH", updateURL, bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("リクエストの作成エラー: %v\n", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"message": "リクエストの作成に失敗しました",
		})
	}

	updateReq.Header.Set("apikey", supabaseKey)
	updateReq.Header.Set("Authorization", "Bearer "+supabaseKey)
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.Header.Set("Prefer", "return=minimal")

	resp, err := http.DefaultClient.Do(updateReq)
	if err != nil {
		fmt.Printf("APIリクエストエラー: %v\n", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"message": "APIリクエストに失敗しました",
		})
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"message": "優先度の更新に失敗しました",
		})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "優先度を更新しました",
	})
}

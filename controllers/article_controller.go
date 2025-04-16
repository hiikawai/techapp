package controllers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
)

type ArticleController struct{}

func NewArticleController() *ArticleController {
	return &ArticleController{}
}

// ShowArticles 過去の記事一覧ページを表示
func (ac *ArticleController) ShowArticles(c echo.Context) error {
	sess, err := session.Get("login-session", c)
	if err != nil {
		return c.Redirect(http.StatusSeeOther, "/")
	}

	// セッションの検証
	auth, ok := sess.Values["authenticated"].(bool)
	if !ok || !auth {
		return c.Redirect(http.StatusSeeOther, "/")
	}

	return c.File("views/articles.html")
}

// GetArticles ルームIDに紐づく記事一覧を取得
func (ac *ArticleController) GetArticles(c echo.Context) error {
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

	// 記事一覧を取得するクエリ
	fullURL := supabaseURL + "/rest/v1/reserve_article?select=*&room_id=eq." + roomID

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

	var articles []map[string]interface{}
	if err := json.Unmarshal(body, &articles); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"message": "JSONのパースに失敗しました",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"articles": articles,
	})
}

// DeleteArticles 選択された記事を削除
func (ac *ArticleController) DeleteArticles(c echo.Context) error {
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
		ArticleIDs []string `json:"article_ids"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"message": "無効なリクエストです: " + err.Error(),
		})
	}

	if len(req.ArticleIDs) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"message": "削除する記事が指定されていません",
		})
	}

	fmt.Printf("削除リクエスト - Room ID: %s, Article IDs: %v\n", roomID, req.ArticleIDs)

	supabaseURL := os.Getenv("SUPABASE_URL")
	supabaseKey := os.Getenv("SUPABASE_KEY")

	// 各記事を削除
	for _, articleID := range req.ArticleIDs {
		deleteURL := fmt.Sprintf("%s/rest/v1/reserve_article?article_id=eq.%s&room_id=eq.%s", supabaseURL, articleID, roomID)
		fmt.Printf("削除URL: %s\n", deleteURL)

		deleteReq, err := http.NewRequest("DELETE", deleteURL, nil)
		if err != nil {
			fmt.Printf("リクエスト作成エラー: %v\n", err)
			continue
		}

		deleteReq.Header.Set("apikey", supabaseKey)
		deleteReq.Header.Set("Authorization", "Bearer "+supabaseKey)
		deleteReq.Header.Set("Content-Type", "application/json")
		deleteReq.Header.Set("Prefer", "return=minimal")

		resp, err := http.DefaultClient.Do(deleteReq)
		if err != nil {
			fmt.Printf("削除リクエストエラー: %v\n", err)
			continue
		}

		if resp.StatusCode != http.StatusNoContent {
			bodyBytes, _ := io.ReadAll(resp.Body)
			fmt.Printf("削除エラー - Status: %d, Body: %s\n", resp.StatusCode, string(bodyBytes))
		} else {
			fmt.Printf("記事ID %s の削除成功\n", articleID)
		}
		resp.Body.Close()
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "選択された記事を削除しました",
	})
}

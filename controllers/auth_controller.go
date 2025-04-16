package controllers

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"time"

	"login-app/models"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
)

// AuthController 認証関連のコントローラー
type AuthController struct{}

// NewAuthController コントローラーのインスタンスを作成
func NewAuthController() *AuthController {
	return &AuthController{}
}

// ShowLogin ログインページを表示
func (ac *AuthController) ShowLogin(c echo.Context) error {
	sess, _ := session.Get("login-session", c)
	if auth, ok := sess.Values["authenticated"].(bool); ok && auth {
		return c.Redirect(http.StatusSeeOther, "/admin")
	}
	return c.File("views/login.html")
}

// ShowAdmin 管理ページを表示
func (ac *AuthController) ShowAdmin(c echo.Context) error {
	sess, err := session.Get("login-session", c)
	if err != nil {
		return c.Redirect(http.StatusSeeOther, "/")
	}

	// セッションの各種チェック
	if !ac.validateSession(sess) {
		sess.Options.MaxAge = -1
		sess.Values = make(map[interface{}]interface{})
		sess.Save(c.Request(), c.Response())
		return c.Redirect(http.StatusSeeOther, "/")
	}

	return c.File("views/admin.html")
}

// validateSession セッションの検証
func (ac *AuthController) validateSession(sess *sessions.Session) bool {
	// 1. 認証状態のチェック
	auth, ok := sess.Values["authenticated"].(bool)
	if !ok || !auth {
		return false
	}

	// 2. CSRFトークンの存在チェック
	if sess.Values["csrf_token"] == nil {
		return false
	}

	// 3. room_idの存在チェック
	if sess.Values["room_id"] == nil {
		return false
	}

	// 4. ログイン時刻のチェック
	loginTime, ok := sess.Values["login_time"].(int64)
	if !ok {
		return false
	}

	// 5. セッション有効期限のチェック（30分）
	if time.Now().Unix()-loginTime > 1800 {
		return false
	}

	// 6. セッションIDの存在チェック
	if sess.Values["session_id"] == nil {
		return false
	}

	return true
}

// Login ログイン処理
func (ac *AuthController) Login(c echo.Context) error {
	user := &models.User{
		RoomID: c.FormValue("room_id"),
	}

	if user.Authenticate() {
		sess, _ := session.Get("login-session", c)

		// セッションの設定
		sess.Options = &sessions.Options{
			Path:     "/",
			MaxAge:   1800, // 30分
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteStrictMode,
		}

		// セッション情報の設定
		sessionID := generateRandomToken()
		token := generateRandomToken()
		loginTime := time.Now().Unix()

		sess.Values["authenticated"] = true
		sess.Values["room_id"] = user.RoomID
		sess.Values["login_time"] = loginTime
		sess.Values["csrf_token"] = token
		sess.Values["session_id"] = sessionID

		if err := sess.Save(c.Request(), c.Response()); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"status":  "error",
				"message": "セッションの保存に失敗しました",
			})
		}

		return c.JSON(http.StatusOK, map[string]string{
			"status":  "success",
			"message": "ログインに成功しました",
		})
	}

	return c.JSON(http.StatusUnauthorized, map[string]string{
		"status":  "error",
		"message": "ログインに失敗しました",
	})
}

// Logout ログアウト処理
func (ac *AuthController) Logout(c echo.Context) error {
	sess, _ := session.Get("login-session", c)

	// セッションを完全に削除
	sess.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	}

	// セッションの全データを削除
	sess.Values = make(map[interface{}]interface{})

	if err := sess.Save(c.Request(), c.Response()); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"status":  "error",
			"message": "ログアウトに失敗しました",
		})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"status":   "success",
		"message":  "ログアウトしました",
		"redirect": "/",
	})
}

// RequireAuth 認証ミドルウェア
func (ac *AuthController) RequireAuth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		sess, err := session.Get("login-session", c)
		if err != nil {
			return c.Redirect(http.StatusSeeOther, "/")
		}

		// セッションの検証
		if !ac.validateSession(sess) {
			// 無効なセッションを削除
			sess.Options.MaxAge = -1
			sess.Values = make(map[interface{}]interface{})
			sess.Save(c.Request(), c.Response())
			return c.Redirect(http.StatusSeeOther, "/")
		}

		return next(c)
	}
}

// generateRandomToken CSRFトークンを生成
func generateRandomToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return base64.URLEncoding.EncodeToString(b)
}

// GetUsers ユーザー一覧を取得
func (ac *AuthController) GetUsers(c echo.Context) error {
	// 認証済みユーザーのみアクセス可能
	sess, _ := session.Get("login-session", c)
	auth, ok := sess.Values["authenticated"].(bool)
	if !ok || !auth {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"message": "認証が必要です",
		})
	}

	supabaseURL := os.Getenv("SUPABASE_URL")
	supabaseKey := os.Getenv("SUPABASE_KEY")

	// URLのデバッグ出力
	fullURL := supabaseURL + "/rest/v1/user?select=room_id"
	println("Request URL:", fullURL)

	userReq, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"message": "リクエストの作成に失敗しました",
		})
	}

	userReq.Header.Set("apikey", supabaseKey)
	userReq.Header.Set("Authorization", "Bearer "+supabaseKey)
	userReq.Header.Set("Content-Type", "application/json")

	// ヘッダーのデバッグ出力
	println("Request Headers:")
	for k, v := range userReq.Header {
		println(k+":", v[0])
	}

	userResp, err := http.DefaultClient.Do(userReq)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"message": "APIリクエストに失敗しました",
		})
	}
	defer userResp.Body.Close()

	// レスポンスステータスのデバッグ出力
	println("Response Status:", userResp.Status)

	userBody, err := io.ReadAll(userResp.Body)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"message": "レスポンスの読み取りに失敗しました",
		})
	}

	// レスポンスボディのデバッグ出力
	println("Response Body:", string(userBody))

	var users []struct {
		RoomID string `json:"room_id"`
	}
	if err := json.Unmarshal(userBody, &users); err != nil {
		// JSONパースエラーの詳細を出力
		println("JSON Parse Error:", err.Error())
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"message": "JSONのパースに失敗しました: " + err.Error(),
		})
	}

	if len(users) == 0 {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"message": "登録されているユーザーがいません",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"users": users,
	})
}

// GetRoomID ログインしているユーザーのroom_idを取得
func (ac *AuthController) GetRoomID(c echo.Context) error {
	sess, err := session.Get("login-session", c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"message": "セッションが無効です",
		})
	}

	// セッションの検証
	if !ac.validateSession(sess) {
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

	return c.JSON(http.StatusOK, map[string]string{
		"room_id": roomID,
	})
}

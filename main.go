package main

import (
	"log"
	"os"

	"login-app/controllers"
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/joho/godotenv"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
)

// セッションのキー
const (
	sessionName = "login-session"
	sessionKey  = "authenticated"
)

func init() {
	// .envファイルの読み込み
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found")
	}
}

func main() {
	e := echo.New()

	// セッションミドルウェアの設定
	sessionSecret := os.Getenv("SESSION_SECRET")
	if sessionSecret == "" {
		sessionSecret = "default-secret-key" // 開発用のデフォルト値
	}
	e.Use(session.Middleware(sessions.NewCookieStore([]byte(sessionSecret))))

	// コントローラーの初期化
	authController := controllers.NewAuthController()
	fieldController := controllers.NewFieldController()
	articleController := controllers.NewArticleController()

	// 静的ファイルの提供（削除）

	// 認証関連のルーティング
	e.GET("/", authController.ShowLogin)
	e.POST("/login", authController.Login)
	e.POST("/logout", authController.Logout)
	e.GET("/get-room-id", authController.GetRoomID, authController.RequireAuth)

	// 分野管理関連のルーティング
	e.GET("/admin", fieldController.ShowFields, authController.RequireAuth)
	e.GET("/api/fields", fieldController.GetFields, authController.RequireAuth)
	e.DELETE("/api/fields", fieldController.DeleteFields, authController.RequireAuth)
	e.POST("/api/fields", fieldController.AddField, authController.RequireAuth)
	e.PUT("/api/fields/priority", fieldController.UpdateFieldPriority, authController.RequireAuth)

	// 記事一覧関連のルーティング
	e.GET("/articles", articleController.ShowArticles, authController.RequireAuth)
	e.GET("/api/articles", articleController.GetArticles, authController.RequireAuth)
	e.DELETE("/api/articles", articleController.DeleteArticles, authController.RequireAuth)

	e.GET("/keepalive", func(c echo.Context) error {
		return c.String(http.StatusOK, "alive!")
	})
	e.HEAD("/keepalive", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	// ポート番号の設定
	port := os.Getenv("PORT")
	if port == "" {
		port = "10000"
	}

	// サーバー起動
	e.Logger.Fatal(e.Start(":" + port))
}

// 認証ミドルウェア
func requireAuth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		sess, err := session.Get(sessionName, c)
		if err != nil {
			return c.Redirect(http.StatusSeeOther, "/")
		}

		auth, ok := sess.Values[sessionKey].(bool)
		if !ok || !auth {
			return c.Redirect(http.StatusSeeOther, "/")
		}

		return next(c)
	}
}

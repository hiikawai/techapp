package models

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
)

// User モデル
type User struct {
	RoomID string
}

// Authenticate ユーザー認証を行う
func (u *User) Authenticate() bool {
	supabaseURL := os.Getenv("SUPABASE_URL")
	supabaseKey := os.Getenv("SUPABASE_KEY")

	// Supabaseからroom_idのリストを取得
	userReq, err := http.NewRequest("GET", supabaseURL+"/rest/v1/user?select=room_id", nil)
	if err != nil {
		return false
	}

	userReq.Header.Set("apikey", supabaseKey)
	userReq.Header.Set("Authorization", "Bearer "+supabaseKey)
	userReq.Header.Set("Content-Type", "application/json")

	userResp, err := http.DefaultClient.Do(userReq)
	if err != nil {
		return false
	}
	defer userResp.Body.Close()

	userBody, err := io.ReadAll(userResp.Body)
	if err != nil {
		return false
	}

	var users []struct {
		RoomID string `json:"room_id"`
	}
	if err := json.Unmarshal(userBody, &users); err != nil {
		return false
	}

	// 入力されたroom_idが存在するかチェック
	for _, user := range users {
		if user.RoomID == u.RoomID {
			return true
		}
	}

	return false
}

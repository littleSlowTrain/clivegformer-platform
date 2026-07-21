package handler

import (
	"testing"
	"time"

	"github.com/clivegformer/platform/user_srv/model"
)

func TestAuthResponse(t *testing.T) {
	s := &Server{jwtSecret: []byte("test-secret")}
	resp, err := s.authResponse(model.User{ID: 7, Username: "researcher", Email: "r@example.com", Role: "user"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Token == "" || resp.UserId != 7 || resp.ExpiresAt <= time.Now().Unix() {
		t.Fatalf("unexpected response: %#v", resp)
	}
}

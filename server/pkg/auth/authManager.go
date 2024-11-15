package auth

import (
	"context"
)

type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type ChangeCredentials struct {
	OldPassword string `json:"oldPassword"`
	Password    string `json:"password"`
}

type User struct {
	Username string `json:"username"`
	Role     string `json:"role"`
}

type AuthManager interface {
	InitialiseDatabase(ctx context.Context) error
	CreateAccount(ctx context.Context, crednetials *Credentials) error
	Login(ctx context.Context, crednetials *Credentials) (string, error)
	IsValidLogin(ctx context.Context, token string) (string, error)
	RevokeToken(ctx context.Context, tokenString string) error
	GetUsers(ctx context.Context, limit, offset int) ([]*User, error)
	GetUserPageCount(ctx context.Context, limit int) (int, error)
	DeleteUser(ctx context.Context, user string) error
	ChangePassword(ctx context.Context, username string, changeCredentials ChangeCredentials) error
}

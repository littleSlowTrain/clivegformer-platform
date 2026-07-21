package handler

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	userv1 "github.com/clivegformer/platform/contracts/gen/user/v1"
	"github.com/clivegformer/platform/user_srv/model"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

type Server struct {
	userv1.UnimplementedUserServiceServer
	db        *gorm.DB
	jwtSecret []byte
}

func New(db *gorm.DB, jwtSecret string) *Server { return &Server{db: db, jwtSecret: []byte(jwtSecret)} }

func (s *Server) Register(ctx context.Context, req *userv1.RegisterRequest) (*userv1.AuthResponse, error) {
	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if len(req.Username) < 3 || len(req.Username) > 64 || len(req.Password) < 8 || !strings.Contains(req.Email, "@") {
		return nil, status.Error(codes.InvalidArgument, "用户名、邮箱或密码格式不正确")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, status.Error(codes.Internal, "密码处理失败")
	}
	user := model.User{Username: req.Username, Email: req.Email, PasswordHash: string(hash), Role: "user"}
	if err := s.db.WithContext(ctx).Create(&user).Error; err != nil {
		return nil, status.Error(codes.AlreadyExists, "用户名或邮箱已存在")
	}
	return s.authResponse(user)
}

func (s *Server) Login(ctx context.Context, req *userv1.LoginRequest) (*userv1.AuthResponse, error) {
	var user model.User
	err := s.db.WithContext(ctx).Where("username = ?", strings.TrimSpace(req.Username)).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
		return nil, status.Error(codes.Unauthenticated, "用户名或密码错误")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "登录失败")
	}
	return s.authResponse(user)
}

func (s *Server) authResponse(user model.User) (*userv1.AuthResponse, error) {
	expires := time.Now().Add(2 * time.Hour)
	claims := jwt.MapClaims{"sub": strconv.FormatUint(user.ID, 10), "username": user.Username, "role": user.Role, "exp": expires.Unix(), "iat": time.Now().Unix()}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.jwtSecret)
	if err != nil {
		return nil, status.Error(codes.Internal, "令牌签发失败")
	}
	return &userv1.AuthResponse{UserId: user.ID, Username: user.Username, Email: user.Email, Role: user.Role, Token: token, ExpiresAt: expires.Unix()}, nil
}

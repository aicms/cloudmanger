package service

import (
	"errors"

	"cloudmanager/internal/auth"
	"cloudmanager/internal/model"
	"cloudmanager/internal/repository"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthService struct {
	db  *gorm.DB
	jwt *auth.JWTManager
}

func NewAuthService(db *gorm.DB, jwt *auth.JWTManager) *AuthService {
	return &AuthService{db: db, jwt: jwt}
}

// Login 校验密码并签发 token
func (s *AuthService) Login(req model.LoginRequest) (*model.LoginResponse, error) {
	repo := repository.NewUserRepository(s.db)
	u, err := repo.FindByUsername(req.Username)
	if err != nil {
		return nil, errors.New("用户名或密码错误")
	}
	if u.Status != "active" {
		return nil, errors.New("账号已禁用")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)); err != nil {
		return nil, errors.New("用户名或密码错误")
	}
	token, err := s.jwt.Generate(u.ID, u.Username, u.Role)
	if err != nil {
		return nil, err
	}
	return &model.LoginResponse{
		Token:     token,
		ExpiresIn: 86400,
		User:      *u,
	}, nil
}

func (s *AuthService) GetProfile(userID uint) (*model.User, error) {
	repo := repository.NewUserRepository(s.db)
	return repo.FindByID(userID)
}

func (s *AuthService) ChangePassword(userID uint, oldPwd, newPwd string) error {
	repo := repository.NewUserRepository(s.db)
	u, err := repo.FindByID(userID)
	if err != nil {
		return errors.New("用户不存在")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(oldPwd)); err != nil {
		return errors.New("原密码错误")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPwd), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return repo.UpdatePassword(userID, string(hash))
}

// HashPassword 工具函数（种子数据用）
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

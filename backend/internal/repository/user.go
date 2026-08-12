package repository

import (
	"cloudmanager/internal/model"

	"gorm.io/gorm"
)

type UserRepository struct{ db *gorm.DB }

func NewUserRepository(db *gorm.DB) *UserRepository { return &UserRepository{db: db} }

func (r *UserRepository) FindByUsername(username string) (*model.User, error) {
	var u model.User
	if err := r.db.Where("username = ?", username).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) FindByID(id uint) (*model.User, error) {
	var u model.User
	if err := r.db.First(&u, id).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) Create(u *model.User) error {
	return r.db.Create(u).Error
}

func (r *UserRepository) UpdatePassword(id uint, hash string) error {
	return r.db.Model(&model.User{}).Where("id = ?", id).Update("password_hash", hash).Error
}

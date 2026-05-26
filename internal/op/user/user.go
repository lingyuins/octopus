package user

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"gorm.io/gorm"
)

var adminCache model.User

const minInitialAdminPasswordLength = 12

var (
	ErrBootstrapAlreadySetUp = errors.New("initial admin account is already set up")
	ErrBootstrapCredentials  = errors.New("invalid bootstrap credentials")
)

// GetAdminCache returns the cached admin user (for backward compatibility).
func GetAdminCache() model.User { return adminCache }

// SetCache sets the admin cache value (for backward compatibility with tests).
func SetCache(u model.User) { adminCache = u }

func Ready() bool {
	return adminCache.ID != 0
}

func BootstrapStatus() (bool, string, error) {
	if Ready() {
		return true, "", nil
	}

	var count int64
	if err := db.GetDB().Model(&model.User{}).Count(&count).Error; err != nil {
		if errors.Is(err, gorm.ErrInvalidDB) {
			return false, "database not initialized", err
		}
		return false, "failed to inspect user initialization state", fmt.Errorf("count users: %w", err)
	}
	if count > 0 {
		return true, "", nil
	}
	return false, "initial admin account is not set up yet", nil
}

// DeleteLegacyAdmin deletes the legacy admin user.
func DeleteLegacyAdmin(targetUsername string) error {
	if targetUsername == "admin" {
		return nil
	}

	result := db.GetDB().Where("username = ?", "admin").Delete(&model.User{})
	if result.Error != nil {
		return fmt.Errorf("delete legacy admin user: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil
	}

	if adminCache.Username == "admin" {
		adminCache = model.User{}
	}
	return nil
}

func ValidateRole(role string) error {
	if role != model.UserRoleAdmin && role != model.UserRoleEditor && role != model.UserRoleViewer {
		return fmt.Errorf("invalid role: %s", role)
	}
	return nil
}

func Init() error {
	if err := bootstrapFromEnv(); err != nil {
		return err
	}

	result := db.GetDB().First(&adminCache)
	if result.Error == nil {
		return nil
	}
	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return result.Error
	}

	adminCache = model.User{}
	return nil
}

func bootstrapFromEnv() error {
	username := strings.TrimSpace(os.Getenv("OCTOPUS_INITIAL_ADMIN_USERNAME"))
	password := os.Getenv("OCTOPUS_INITIAL_ADMIN_PASSWORD")

	if username == "" && password == "" {
		return nil
	}
	if username == "" || password == "" {
		return fmt.Errorf("both OCTOPUS_INITIAL_ADMIN_USERNAME and OCTOPUS_INITIAL_ADMIN_PASSWORD must be set together")
	}

	if err := deleteLegacyAdmin(username); err != nil {
		return err
	}

	if Ready() && adminCache.Username == username {
		return nil
	}

	if err := BootstrapCreate(username, password); err != nil {
		return fmt.Errorf("bootstrap admin from env: %w", err)
	}
	return nil
}

func deleteLegacyAdmin(targetUsername string) error {
	if targetUsername == "admin" {
		return nil
	}

	result := db.GetDB().Where("username = ?", "admin").Delete(&model.User{})
	if result.Error != nil {
		return fmt.Errorf("delete legacy admin user: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil
	}

	if adminCache.Username == "admin" {
		adminCache = model.User{}
	}
	return nil
}

func BootstrapCreate(username, password string) error {
	if err := validateManagedCredentials(username, password); err != nil {
		return err
	}
	username = strings.TrimSpace(username)

	var count int64
	if err := db.GetDB().Model(&model.User{}).Count(&count).Error; err != nil {
		return fmt.Errorf("failed to inspect user state: %w", err)
	}
	if count > 0 {
		return ErrBootstrapAlreadySetUp
	}

	user := model.User{
		Username: username,
		Password: password,
	}
	if err := user.HashPassword(); err != nil {
		return err
	}
	if err := db.GetDB().Create(&user).Error; err != nil {
		return err
	}
	adminCache = user
	return nil
}

func validateManagedCredentials(username, password string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return fmt.Errorf("%w: username is required", ErrBootstrapCredentials)
	}
	if password == "" {
		return fmt.Errorf("%w: password is required", ErrBootstrapCredentials)
	}
	if utf8.RuneCountInString(password) < minInitialAdminPasswordLength {
		return fmt.Errorf("%w: password must be at least %d characters long", ErrBootstrapCredentials, minInitialAdminPasswordLength)
	}
	return nil
}

func Create(req model.UserCreateRequest, ctx context.Context) error {
	req.Username = strings.TrimSpace(req.Username)
	if err := validateManagedCredentials(req.Username, req.Password); err != nil {
		return err
	}
	if err := ValidateRole(req.Role); err != nil {
		return err
	}

	var count int64
	if err := db.GetDB().WithContext(ctx).Model(&model.User{}).
		Where("username = ?", req.Username).
		Count(&count).Error; err != nil {
		return fmt.Errorf("failed to inspect existing users: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("username already exists")
	}

	user := model.User{
		Username: req.Username,
		Password: req.Password,
		Role:     req.Role,
	}
	if err := user.HashPassword(); err != nil {
		return err
	}
	if err := db.GetDB().WithContext(ctx).Create(&user).Error; err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

func ChangePassword(userID uint, oldPassword, newPassword string) error {
	user, err := GetByID(userID, context.Background())
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}
	if err := user.ComparePassword(oldPassword); err != nil {
		return fmt.Errorf("incorrect old password: %w", err)
	}

	user.Password = newPassword
	if err := user.HashPassword(); err != nil {
		return fmt.Errorf("failed to hash new password: %w", err)
	}

	if err := db.GetDB().Model(&user).Update("password", user.Password).Error; err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}
	if adminCache.ID == user.ID {
		adminCache.Password = user.Password
	}
	return nil
}

func ChangeUsername(userID uint, newUsername string) error {
	newUsername = strings.TrimSpace(newUsername)
	if newUsername == "" {
		return fmt.Errorf("username is required")
	}

	user, err := GetByID(userID, context.Background())
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}
	if user.Username == newUsername {
		return fmt.Errorf("new username is the same as the old username")
	}

	var count int64
	if err := db.GetDB().Model(&model.User{}).
		Where("username = ? AND id <> ?", newUsername, user.ID).
		Count(&count).Error; err != nil {
		return fmt.Errorf("failed to inspect existing users: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("username already exists")
	}

	user.Username = newUsername
	if err := db.GetDB().Model(&user).Update("username", user.Username).Error; err != nil {
		return fmt.Errorf("failed to update username: %w", err)
	}
	if adminCache.ID == user.ID {
		adminCache.Username = user.Username
	}
	return nil
}

func Verify(username, password string) (model.User, error) {
	if !Ready() {
		return model.User{}, fmt.Errorf("user not initialized: %w", ErrBootstrapAlreadySetUp)
	}
	user, err := GetByUsername(strings.TrimSpace(username), context.Background())
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.User{}, fmt.Errorf("incorrect username")
		}
		return model.User{}, fmt.Errorf("failed to load user: %w", err)
	}
	if err := user.ComparePassword(password); err != nil {
		return model.User{}, fmt.Errorf("incorrect password")
	}
	return user, nil
}

func GetCurrent() model.User {
	return adminCache
}

func GetByID(id uint, ctx context.Context) (model.User, error) {
	var user model.User
	if err := db.GetDB().WithContext(ctx).First(&user, id).Error; err != nil {
		return model.User{}, err
	}
	return user, nil
}

func GetByUsername(username string, ctx context.Context) (model.User, error) {
	var user model.User
	if err := db.GetDB().WithContext(ctx).
		Where("username = ?", username).
		First(&user).Error; err != nil {
		return model.User{}, err
	}
	return user, nil
}

func List(ctx context.Context) ([]model.User, error) {
	var users []model.User
	if err := db.GetDB().WithContext(ctx).Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func UpdateRole(id uint, role string, ctx context.Context) error {
	if err := ValidateRole(role); err != nil {
		return err
	}
	res := db.GetDB().WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Update("role", role)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("user not found")
	}
	if adminCache.ID == id {
		adminCache.Role = role
	}
	return nil
}

func Delete(id uint, currentUserID uint, ctx context.Context) error {
	if currentUserID != 0 && id == currentUserID {
		return fmt.Errorf("cannot delete the active user")
	}
	res := db.GetDB().WithContext(ctx).Delete(&model.User{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("user not found")
	}
	if adminCache.ID == id {
		_ = Init()
	}
	return nil
}

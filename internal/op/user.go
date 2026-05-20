package op

import (
	"context"

	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/user"
)

// userCache is retained for backward compatibility (used by tests).
// Internal/op/user is the canonical cache; userCache is synced on each call.
var userCache model.User

// Deprecated: Use user.Ready from internal/op/user instead.
func UserReady() bool { return user.Ready() }

// Deprecated: Use user.BootstrapStatus from internal/op/user instead.
func UserBootstrapStatus() (bool, string, error) { return user.BootstrapStatus() }

// Deprecated: Use user.GetCurrent from internal/op/user instead.
func UserGet() model.User { userCache = user.GetCurrent(); return userCache }

// Deprecated: Use user.GetByID from internal/op/user instead.
func UserGetByID(id uint, ctx context.Context) (model.User, error) { return user.GetByID(id, ctx) }

// Deprecated: Use user.GetByUsername from internal/op/user instead.
func UserGetByUsername(username string, ctx context.Context) (model.User, error) {
	return user.GetByUsername(username, ctx)
}

// Deprecated: Use user.List from internal/op/user instead.
func UserList(ctx context.Context) ([]model.User, error) { return user.List(ctx) }

// Deprecated: Use user.Create from internal/op/user instead.
func UserCreate(req model.UserCreateRequest, ctx context.Context) error {
	return user.Create(req, ctx)
}

// Deprecated: Use user.UpdateRole from internal/op/user instead.
func UserUpdateRole(id uint, role string, ctx context.Context) error {
	return user.UpdateRole(id, role, ctx)
}

// Deprecated: Use user.Delete from internal/op/user instead.
func UserDelete(id uint, currentUserID uint, ctx context.Context) error {
	return user.Delete(id, currentUserID, ctx)
}

// Deprecated: Use user.ChangePassword from internal/op/user instead.
func UserChangePassword(userID uint, oldPassword, newPassword string) error {
	return user.ChangePassword(userID, oldPassword, newPassword)
}

// Deprecated: Use user.ChangeUsername from internal/op/user instead.
func UserChangeUsername(userID uint, newUsername string) error {
	return user.ChangeUsername(userID, newUsername)
}

// Deprecated: Use user.Verify from internal/op/user instead.
func UserVerify(username, password string) (model.User, error) {
	return user.Verify(username, password)
}

// Deprecated: Use user.Init from internal/op/user instead.
func UserInit() error {
	user.SetCache(userCache) // push test-modified value to subpackage
	err := user.Init()
	userCache = user.GetCurrent() // pull canonical value back
	return err
}

// Deprecated: Use user.BootstrapCreate from internal/op/user instead.
func UserBootstrapCreate(username, password string) error {
	user.SetCache(userCache) // push test-modified value to subpackage
	err := user.BootstrapCreate(username, password)
	userCache = user.GetCurrent()
	return err
}

// deleteLegacyAdminUser is retained for backward compatibility (used by tests).
func deleteLegacyAdminUser(targetUsername string) error {
	user.SetCache(userCache) // push test-modified value
	err := user.DeleteLegacyAdmin(targetUsername)
	userCache = user.GetCurrent()
	return err
}

// ErrUserNotInitialized is retained for backward compatibility.
var ErrUserNotInitialized = user.ErrBootstrapAlreadySetUp

// ErrBootstrapAlreadySetUp is retained for backward compatibility.
var ErrBootstrapAlreadySetUp = user.ErrBootstrapAlreadySetUp

// ErrBootstrapCredentials is retained for backward compatibility.
var ErrBootstrapCredentials = user.ErrBootstrapCredentials

package node

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/lingyuins/octopus/internal/db"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/setting"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	ErrInvalidName     = errors.New("invalid node name")
	ErrUnauthorized    = errors.New("invalid node credentials")
	ErrReferenced      = errors.New("node is referenced by channels")
	ErrFeatureDisabled = errors.New("magpie nodes feature is disabled")
	lifecycleMu        sync.Mutex
)

// NodeRevokedFunc is wired at startup. It must synchronously remove the current
// session, must not call back into this package, and must not block on peer I/O.
var NodeRevokedFunc func(int)

// DisconnectAllFunc removes every session without permanently closing the manager.
// Like NodeRevokedFunc, it must not call back into node lifecycle operations.
var DisconnectAllFunc func()

type RuntimeState struct {
	Online           bool       `json:"online"`
	LastSeenAt       *time.Time `json:"last_seen_at"`
	ConnectorVersion string     `json:"connector_version"`
	InFlight         int        `json:"in_flight"`
	MaxConcurrency   int        `json:"max_concurrency"`
	LastError        string     `json:"last_error"`
}

type Record struct {
	model.Node
	RuntimeState
}

type TokenResult struct {
	Node  Record `json:"node"`
	Token string `json:"token"`
}

// RuntimeSnapshotFunc must not acquire the node lifecycle lock.
var RuntimeSnapshotFunc func(int) RuntimeState

func record(n model.Node) Record {
	r := Record{Node: n}
	if RuntimeSnapshotFunc != nil {
		r.RuntimeState = RuntimeSnapshotFunc(n.ID)
	}
	return r
}

// FeatureEnabled reports whether the Magpie nodes feature is globally enabled.
// Errors reading the setting default to enabled so a cold cache never silently
// breaks existing deployments.
func FeatureEnabled() bool {
	enabled, err := setting.GetBool(model.SettingKeyNodesFeatureEnabled)
	if err != nil {
		return true
	}
	return enabled
}

func secretDB(ctx context.Context) *gorm.DB {
	// Even debug SQL must never log credential digests or token lookup arguments.
	return db.GetDB().WithContext(ctx).Session(&gorm.Session{Logger: logger.Discard})
}

// WithBindingLock keeps channel reference validation and writes atomic with node
// deletion. The callback must not invoke another lifecycle-mutating node method.
func WithBindingLock(fn func() error) error {
	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()
	return fn()
}

func Get(ctx context.Context, id int) (*Record, error) {
	var n model.Node
	if err := secretDB(ctx).First(&n, id).Error; err != nil {
		return nil, err
	}
	r := record(n)
	return &r, nil
}

func List(ctx context.Context) ([]Record, error) {
	var nodes []model.Node
	if err := secretDB(ctx).Order("id").Find(&nodes).Error; err != nil {
		return nil, err
	}
	out := make([]Record, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, record(n))
	}
	return out, nil
}

func validName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 200 {
		return "", ErrInvalidName
	}
	return name, nil
}

func generateToken() (string, string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", "", err
	}
	token := "oct_node_" + base64.RawURLEncoding.EncodeToString(b[:])
	return token, hashToken(token), nil
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func Create(ctx context.Context, name string, enabled bool, maxConcurrency int) (*TokenResult, error) {
	name, err := validName(name)
	if err != nil {
		return nil, err
	}
	if maxConcurrency < 1 || maxConcurrency > 32 {
		maxConcurrency = 16
	}
	token, hash, err := generateToken()
	if err != nil {
		return nil, err
	}
	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()
	n := model.Node{Name: name, Enabled: enabled, MaxConcurrency: maxConcurrency, TokenHash: hash}
	if err := secretDB(ctx).Create(&n).Error; err != nil {
		return nil, err
	}
	return &TokenResult{Node: record(n), Token: token}, nil
}

func revoke(id int) {
	if NodeRevokedFunc != nil {
		NodeRevokedFunc(id)
	}
}

func Update(ctx context.Context, id int, name *string, enabled *bool, maxConcurrency *int) (*Record, error) {
	var normalized string
	var err error
	if name != nil {
		normalized, err = validName(*name)
		if err != nil {
			return nil, err
		}
	}
	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()
	var n model.Node
	conn := secretDB(ctx)
	if err := conn.First(&n, id).Error; err != nil {
		return nil, err
	}
	updates := map[string]any{}
	if name != nil {
		updates["name"] = normalized
	}
	if enabled != nil {
		updates["enabled"] = *enabled
	}
	if maxConcurrency != nil {
		mc := *maxConcurrency
		if mc < 1 || mc > 32 {
			mc = 16
		}
		updates["max_concurrency"] = mc
	}
	if len(updates) > 0 {
		if err := conn.Model(&n).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	if enabled != nil && !*enabled {
		revoke(id)
	}
	if err := conn.First(&n, id).Error; err != nil {
		return nil, err
	}
	r := record(n)
	return &r, nil
}

func RotateToken(ctx context.Context, id int) (*TokenResult, error) {
	token, hash, err := generateToken()
	if err != nil {
		return nil, err
	}
	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()
	var n model.Node
	conn := secretDB(ctx)
	if err := conn.First(&n, id).Error; err != nil {
		return nil, err
	}
	if err := conn.Model(&n).Update("token_hash", hash).Error; err != nil {
		return nil, err
	}
	revoke(id)
	return &TokenResult{Node: record(n), Token: token}, nil
}

func Delete(ctx context.Context, id int) error {
	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()
	conn := secretDB(ctx)
	err := conn.Transaction(func(tx *gorm.DB) error {
		var n model.Node
		if err := tx.First(&n, id).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&model.Channel{}).Where("node_id = ?", id).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return ErrReferenced
		}
		return tx.Delete(&n).Error
	})
	if err == nil {
		revoke(id)
	}
	return err
}

// Authenticate does not acquire lifecycleMu: the transport manager invokes the
// final validation while holding its registration lock. Mutations commit before
// acquiring that lock to disconnect, so no revoked credential can be published
// after revocation returns. Never call it from a DB transaction holding a writer.
func Authenticate(ctx context.Context, token string) (int, error) {
	if !FeatureEnabled() {
		return 0, ErrFeatureDisabled
	}
	if !strings.HasPrefix(token, "oct_node_") || len(token) != 52 {
		return 0, ErrUnauthorized
	}
	var n model.Node
	if err := secretDB(ctx).Where("token_hash = ? AND enabled = ?", hashToken(token), true).First(&n).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, ErrUnauthorized
		}
		return 0, err
	}
	return n.ID, nil
}

func ValidateCredential(ctx context.Context, id int, token string) bool {
	actual, err := Authenticate(ctx, token)
	return err == nil && actual == id
}

// WithRestoreLock serializes live restoration with management changes. Disconnect
// only after the restore transaction releases DB locks (manager validation reads
// the DB under its registration lock). Revoke even on partial post-commit errors.
func WithRestoreLock(restore func() ([]int, error)) error {
	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()
	ids, err := restore()
	for _, id := range ids {
		revoke(id)
	}
	return err
}

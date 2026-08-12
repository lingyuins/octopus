package op

import (
	"os"
	"testing"

	"github.com/lingyuins/octopus/internal/utils/crypto"
)

// TestMain 在测试环境初始化加密密钥。生产环境由 cmd/start.go 在启动时
// 强制初始化（未配置密钥拒绝启动）；测试直接给出固定密钥。
func TestMain(m *testing.M) {
	crypto.Init("octopus-test-encryption-key")
	os.Exit(m.Run())
}

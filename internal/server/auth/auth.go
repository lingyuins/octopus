package auth

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/lingyuins/octopus/internal/conf"
	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/op/setting"
)

type jwtClaims struct {
	jwt.RegisteredClaims
	UserID uint   `json:"user_id,omitempty"`
	Role   string `json:"role,omitempty"`
}

// maxJWTExpiryMinutes 是 JWT 有效期的硬上限（30 天）。expire 参数来自客户端登录
// 请求，此前任何非 0/-1 的取值都不会设置 ExpiresAt（无 exp 的 token 永久有效），
// 正值也没有上限（如 5256000 = 10 年）。这里统一钳制：-1 = 记住我，>0 钳制到上限，
// 其余非法值回退到默认有效期——保证任何路径都必然设置 ExpiresAt。
const maxJWTExpiryMinutes = 30 * 24 * 60

func GenerateJWTToken(expiresMin int, userID uint, role string) (string, string, error) {
	now := time.Now()
	claims := &jwtClaims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    conf.APP_NAME,
		},
	}
	expiry := now
	if expiresMin == -1 {
		// 记住我：默认 30 天，受 setting 控制，仍受硬上限约束。
		rememberDays := 30
		if v, err := setting.GetInt(model.SettingKeyJWTRememberMeExpiryDays); err == nil && v > 0 {
			rememberDays = v
		}
		expiry = expiry.Add(time.Duration(rememberDays) * 24 * time.Hour)
	} else if expiresMin > 0 {
		// 自定义有效期：钳制到硬上限，防止客户端申请超长 token。
		if expiresMin > maxJWTExpiryMinutes {
			expiresMin = maxJWTExpiryMinutes
		}
		expiry = expiry.Add(time.Duration(expiresMin) * time.Minute)
	} else {
		// 0 或缺省/非法值：默认有效期（默认 15 分钟，受 setting 控制）。
		defaultExpiry := 15
		if v, err := setting.GetInt(model.SettingKeyJWTDefaultExpiryMinutes); err == nil && v > 0 {
			if v > maxJWTExpiryMinutes {
				v = maxJWTExpiryMinutes
			}
			defaultExpiry = v
		}
		expiry = expiry.Add(time.Duration(defaultExpiry) * time.Minute)
	}
	claims.ExpiresAt = jwt.NewNumericDate(expiry)
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(conf.AppConfig.Auth.JWTSecret))
	if err != nil {
		return "", "", err
	}
	return token, claims.ExpiresAt.Format(time.RFC3339), nil
}

// VerifyJWTToken validates the JWT and returns the user identity in claims.
// 校验时锁定签名算法（HS256）与 Issuer，防止算法混淆与跨实例 token 复用。
func VerifyJWTToken(token string) (bool, uint, string) {
	claims := &jwtClaims{}
	jwtToken, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(conf.AppConfig.Auth.JWTSecret), nil
	},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(conf.APP_NAME),
	)
	if err != nil || !jwtToken.Valid {
		return false, 0, ""
	}
	if claims.Role == "" || claims.UserID == 0 {
		return false, 0, ""
	}
	return true, claims.UserID, claims.Role
}

func GenerateAPIKey() (string, error) {
	const keyChars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, 48)
	maxI := big.NewInt(int64(len(keyChars)))
	for i := range b {
		n, err := rand.Int(rand.Reader, maxI)
		if err != nil {
			return "", fmt.Errorf("generate api key: %w", err)
		}
		b[i] = keyChars[n.Int64()]
	}
	return "sk-" + conf.APP_NAME + "-" + string(b), nil
}

package auth

import (
	"crypto/rand"
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
	if expiresMin == 0 {
		defaultExpiry := 15
		if v, err := setting.GetInt(model.SettingKeyJWTDefaultExpiryMinutes); err == nil && v > 0 {
			defaultExpiry = v
		}
		claims.ExpiresAt = jwt.NewNumericDate(now.Add(time.Duration(defaultExpiry) * time.Minute))
	} else if expiresMin > 0 {
		claims.ExpiresAt = jwt.NewNumericDate(now.Add(time.Duration(expiresMin) * time.Minute))
	} else if expiresMin == -1 {
		rememberDays := 30
		if v, err := setting.GetInt(model.SettingKeyJWTRememberMeExpiryDays); err == nil && v > 0 {
			rememberDays = v
		}
		claims.ExpiresAt = jwt.NewNumericDate(now.Add(time.Duration(rememberDays) * 24 * time.Hour))
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(conf.AppConfig.Auth.JWTSecret))
	if err != nil {
		return "", "", err
	}
	return token, claims.ExpiresAt.Format(time.RFC3339), nil
}

// VerifyJWTToken validates the JWT and returns the user identity in claims.
func VerifyJWTToken(token string) (bool, uint, string) {
	claims := &jwtClaims{}
	jwtToken, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(conf.AppConfig.Auth.JWTSecret), nil
	})
	if err != nil || !jwtToken.Valid {
		return false, 0, ""
	}
	if claims.Role == "" || claims.UserID == 0 {
		return false, 0, ""
	}
	return true, claims.UserID, claims.Role
}

func GenerateAPIKey() string {
	const keyChars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, 48)
	maxI := big.NewInt(int64(len(keyChars)))
	for i := range b {
		n, err := rand.Int(rand.Reader, maxI)
		if err != nil {
			return ""
		}
		b[i] = keyChars[n.Int64()]
	}
	return "sk-" + conf.APP_NAME + "-" + string(b)
}

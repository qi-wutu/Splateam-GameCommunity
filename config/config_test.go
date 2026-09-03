package config

import (
	"strings"
	"testing"
)

// TestValidateJWT 校验 JWT 密钥强度：空、公开默认值、过短都必须拒绝启动。
func TestValidateJWT(t *testing.T) {
	old := JwtSecret
	defer func() { JwtSecret = old }()

	cases := []struct {
		name    string
		secret  string
		wantErr bool
	}{
		{"empty", "", true},
		{"default-secret", "splatoon-dev-secret-key", true},
		{"too-short", "short", true},
		{"strong-32", strings.Repeat("x", 32), false},
		{"strong-64", strings.Repeat("y", 64), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			JwtSecret = c.secret
			err := ValidateJWT()
			if (err != nil) != c.wantErr {
				t.Fatalf("ValidateJWT(%q) error = %v, wantErr=%v", c.secret, err, c.wantErr)
			}
		})
	}
}

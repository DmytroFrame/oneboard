// Package bcrypthash implements auth.PasswordHasher with bcrypt.
package bcrypthash

import (
	"go.uber.org/fx"
	"golang.org/x/crypto/bcrypt"

	"github.com/DmytroFrame/oneboard/backend/internal/usecase/auth"
)

var Module = fx.Module("bcrypt", fx.Provide(func() auth.PasswordHasher { return Hasher{} }))

type Hasher struct{}

func (Hasher) Hash(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(h), err
}

func (Hasher) Compare(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

package auth

import "golang.org/x/crypto/bcrypt"

const bcryptCost = 10

// DummyHash is a pre-computed bcrypt hash used to defend against timing
// attacks during login. When a username doesn't exist, the handler verifies
// the submitted password against this hash so the response time matches the
// "user found" branch. The plaintext does not need to be secret — the goal
// is constant-time response, not hash confidentiality.
const DummyHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func VerifyPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

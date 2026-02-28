package postgres

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestHashPassword(t *testing.T) {
	hash, err := HashPassword("secret123")
	assert.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, "secret123", hash)
}

func TestCheckPassword_Correct(t *testing.T) {
	hash, err := HashPassword("mypassword")
	assert.NoError(t, err)
	assert.True(t, CheckPassword("mypassword", hash))
}

func TestCheckPassword_Wrong(t *testing.T) {
	hash, err := HashPassword("mypassword")
	assert.NoError(t, err)
	assert.False(t, CheckPassword("wrongpassword", hash))
}

// Property: HashPassword always produces a hash that CheckPassword verifies.
func TestPropertyHashAndCheck(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// bcrypt has a max input length of 72 bytes
		password := rapid.StringMatching(`[a-zA-Z0-9!@#$%^&*]{1,64}`).Draw(t, "password")
		hash, err := HashPassword(password)
		if err != nil {
			t.Fatalf("HashPassword failed: %v", err)
		}
		if !CheckPassword(password, hash) {
			t.Fatalf("CheckPassword failed for password %q", password)
		}
	})
}

// TestValidRole verifies the three known roles and rejects unknowns.
func TestValidRole(t *testing.T) {
	assert.True(t, ValidRole(RolePlayer))
	assert.True(t, ValidRole(RoleEditor))
	assert.True(t, ValidRole(RoleAdmin))
	assert.False(t, ValidRole(""))
	assert.False(t, ValidRole("superadmin"))
}

// Property: ValidRole accepts exactly the three defined roles.
func TestPropertyValidRole(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		role := rapid.StringMatching(`[a-z]{1,20}`).Draw(t, "role")
		got := ValidRole(role)
		want := role == RolePlayer || role == RoleEditor || role == RoleAdmin
		if got != want {
			t.Fatalf("ValidRole(%q) = %v, want %v", role, got, want)
		}
	})
}

// Property: Different passwords produce different hashes (probabilistic, but bcrypt
// includes salt, so even same passwords produce different hashes).
func TestPropertyDifferentPasswordsDifferentHashes(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		p1 := rapid.StringMatching(`[a-zA-Z]{6,20}`).Draw(t, "password1")
		p2 := rapid.StringMatching(`[a-zA-Z]{6,20}`).Draw(t, "password2")

		h1, err := HashPassword(p1)
		assert.NoError(t, err)
		h2, err := HashPassword(p2)
		assert.NoError(t, err)

		// Hashes should always differ due to unique salts
		assert.NotEqual(t, h1, h2, "bcrypt hashes should differ due to unique salts")
	})
}

// Property: Wrong password never validates.
func TestPropertyWrongPasswordNeverValidates(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		correct := rapid.StringMatching(`[a-zA-Z0-9]{6,30}`).Draw(t, "correct")
		wrong := rapid.StringMatching(`[a-zA-Z0-9]{6,30}`).Draw(t, "wrong")

		if correct == wrong {
			return // skip trivial case
		}

		hash, err := HashPassword(correct)
		assert.NoError(t, err)
		assert.False(t, CheckPassword(wrong, hash),
			"wrong password %q should not match hash of %q", wrong, correct)
	})
}

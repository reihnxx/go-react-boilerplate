package password

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestHashAndVerify(t *testing.T) {
	h, err := Hash("rahasia123")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "$argon2id$v=19$") {
		t.Fatalf("unexpected format %q", h)
	}
	ok, rehash, err := Verify("rahasia123", h)
	if err != nil || !ok || rehash {
		t.Fatalf("Verify(correct) = %v, %v, %v", ok, rehash, err)
	}
	ok, _, err = Verify("salah123", h)
	if err != nil || ok {
		t.Fatalf("Verify(wrong) = %v, %v", ok, err)
	}
}

func TestHashUsesRandomSalt(t *testing.T) {
	a, _ := Hash("same-password1")
	b, _ := Hash("same-password1")
	if a == b {
		t.Fatal("two hashes of the same password must differ")
	}
}

func TestVerifyLegacyBcryptRequestsRehash(t *testing.T) {
	raw, err := bcrypt.GenerateFromPassword([]byte("laravel123"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	// Laravel stores bcrypt hashes with the $2y$ prefix.
	legacy := "$2y$" + string(raw)[4:]
	ok, rehash, err := Verify("laravel123", legacy)
	if err != nil || !ok || !rehash {
		t.Fatalf("Verify(legacy) = %v, %v, %v", ok, rehash, err)
	}
	ok, _, _ = Verify("other123", legacy)
	if ok {
		t.Fatal("wrong password matched legacy hash")
	}
}

func TestVerifyMalformed(t *testing.T) {
	for _, h := range []string{"", "plain", "$argon2id$v=19$m=1,t=1,p=1$bad$", "$argon2i$v=19$m=1,t=1,p=1$YQ$YQ"} {
		if ok, _, err := Verify("x", h); ok || err == nil {
			t.Errorf("Verify(%q) = %v, %v; want false, error", h, ok, err)
		}
	}
}

func TestVerifyStaleParamsRequestsRehash(t *testing.T) {
	old := current
	current = params{memory: 8 * 1024, iterations: 1, parallelism: 1, saltLen: 16, keyLen: 32}
	h, _ := Hash("parameter1")
	current = old
	ok, rehash, err := Verify("parameter1", h)
	if err != nil || !ok || !rehash {
		t.Fatalf("Verify(stale) = %v, %v, %v", ok, rehash, err)
	}
}

func TestValidate(t *testing.T) {
	cases := map[string]bool{
		"":                       false,
		"abc12":                  false, // too short
		"abcdefgh":               false, // no digit
		"12345678":               false, // no letter
		"abcdefg1":               true,
		"Kata sandi 9":           true,
		strings.Repeat("a1", 65): false, // 130 chars
	}
	for pw, valid := range cases {
		if got := Validate(pw) == ""; got != valid {
			t.Errorf("Validate(%q) valid=%v, want %v", pw, got, valid)
		}
	}
}

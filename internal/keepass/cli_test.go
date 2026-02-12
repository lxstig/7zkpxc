package keepass

import (
	"os/exec"
	"testing"
)

func TestMasterPasswordZeroedOnClose(t *testing.T) {
	c := &Client{
		masterPassword: []byte("supersecret123!@#"),
	}

	// Verify password is set
	if len(c.masterPassword) == 0 {
		t.Fatal("password should be set before Close()")
	}

	// Close should zero the bytes
	c.Close()

	// Verify password is nil
	if c.masterPassword != nil {
		t.Fatal("masterPassword should be nil after Close()")
	}
}

func TestMasterPasswordBytesZeroed(t *testing.T) {
	// Store reference to prove bytes were zeroed in-place
	secret := []byte("supersecret123!@#")
	c := &Client{
		masterPassword: secret,
	}

	c.Close()

	// Verify original bytes were zeroed, not just the reference
	for i, b := range secret {
		if b != 0 {
			t.Fatalf("byte at index %d not zeroed: got %d", i, b)
		}
	}
}

func TestGetMasterPassword(t *testing.T) {
	c := &Client{
		masterPassword: []byte("testpass"),
	}

	got := c.getMasterPassword()
	if got != "testpass" {
		t.Errorf("getMasterPassword() = %q, want %q", got, "testpass")
	}
}

func TestGeneratePassword_RequiresKeepassxcCLI(t *testing.T) {
	if _, err := exec.LookPath("keepassxc-cli"); err != nil {
		t.Skip("keepassxc-cli not installed, skipping")
	}

	client := New("/dummy/path.kdbx")

	pw, err := client.GeneratePassword(64)
	if err != nil {
		t.Fatalf("GeneratePassword(64) failed: %v", err)
	}

	if len(pw) != 64 {
		t.Errorf("GeneratePassword(64) returned length %d, want 64", len(pw))
	}
}

func TestGeneratePassword_Lengths(t *testing.T) {
	if _, err := exec.LookPath("keepassxc-cli"); err != nil {
		t.Skip("keepassxc-cli not installed, skipping")
	}

	client := New("/dummy/path.kdbx")

	tests := []struct {
		name   string
		length int
	}{
		{"Minimum (32)", 32},
		{"Default (64)", 64},
		{"Maximum (128)", 128},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pw, err := client.GeneratePassword(tt.length)
			if err != nil {
				t.Fatalf("GeneratePassword(%d) failed: %v", tt.length, err)
			}

			if len(pw) != tt.length {
				t.Errorf("len = %d, want %d", len(pw), tt.length)
			}

			if pw == "" {
				t.Error("password is empty")
			}
		})
	}
}

func TestGeneratePassword_Uniqueness(t *testing.T) {
	if _, err := exec.LookPath("keepassxc-cli"); err != nil {
		t.Skip("keepassxc-cli not installed, skipping")
	}

	client := New("/dummy/path.kdbx")

	passwords := make(map[string]struct{})
	for i := 0; i < 10; i++ {
		pw, err := client.GeneratePassword(64)
		if err != nil {
			t.Fatalf("GeneratePassword failed on iteration %d: %v", i, err)
		}
		if _, exists := passwords[pw]; exists {
			t.Fatalf("duplicate password generated on iteration %d", i)
		}
		passwords[pw] = struct{}{}
	}
}

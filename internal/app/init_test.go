package app

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestExpandTilde_Home(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Just tilde", "~", home},
		{"Tilde with path", "~/documents", filepath.Join(home, "documents")},
		{"Tilde deep path", "~/a/b/c", filepath.Join(home, "a/b/c")},
		{"No tilde", "/absolute/path", "/absolute/path"},
		{"Relative", "relative/path", "relative/path"},
		{"Tilde in middle", "/path/~/weird", "/path/~/weird"},
		{"Empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandTilde(tt.input)
			if result != tt.expected {
				t.Errorf("expandTilde(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExpandAndResolve(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}

	result := expandAndResolve("~/test.kdbx")
	expected := filepath.Join(home, "test.kdbx")
	if result != expected {
		t.Errorf("expandAndResolve(\"~/test.kdbx\") = %q, want %q", result, expected)
	}

	// Absolute path should stay absolute
	abs := expandAndResolve("/etc/test.kdbx")
	if abs != "/etc/test.kdbx" {
		t.Errorf("expandAndResolve(\"/etc/test.kdbx\") = %q, want %q", abs, "/etc/test.kdbx")
	}
}

func TestKdbxFilter(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		isDir  bool
		expect bool
	}{
		{"KDBX file", "/path/to/test.kdbx", false, true},
		{"KDBX uppercase", "/path/to/Test.KDBX", false, true},
		{"Directory", "/path/to/dir", true, true},
		{"Non-kdbx file", "/path/to/test.txt", false, false},
		{"No extension", "/path/to/test", false, false},
		{"Partial match", "/path/to/test.kdbx.bak", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := kdbxFilter(tt.path, tt.isDir)
			if result != tt.expect {
				t.Errorf("kdbxFilter(%q, %v) = %v, want %v", tt.path, tt.isDir, result, tt.expect)
			}
		})
	}
}

func TestIsDir(t *testing.T) {
	// Existing directory
	tmpDir := t.TempDir()
	if !isDir(tmpDir) {
		t.Errorf("isDir(%q) = false, want true", tmpDir)
	}

	// Existing file
	tmpFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(tmpFile, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if isDir(tmpFile) {
		t.Errorf("isDir(%q) = true, want false", tmpFile)
	}

	// Non-existent path
	if isDir("/nonexistent/path") {
		t.Error("isDir(\"/nonexistent/path\") = true, want false")
	}
}

func TestDetectSevenZipBinary(t *testing.T) {
	name, found := detectSevenZipBinary()
	if runtime.GOOS == "linux" {
		// On a typical dev machine, at least one of 7zz, 7z, 7za should be present
		if found {
			if name != "7zz" && name != "7z" && name != "7za" {
				t.Errorf("detectSevenZipBinary() = %q, expected one of 7zz/7z/7za", name)
			}
		} else {
			// Not found is valid too — just check default
			if name != "7z" {
				t.Errorf("detectSevenZipBinary() fallback = %q, want \"7z\"", name)
			}
		}
	}
}

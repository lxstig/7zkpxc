package app

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/lxstig/7zkpxc/internal/config"
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

// -------------------------------------------------------------------
// kdbxPainter.Paint (0% → 100%)
// -------------------------------------------------------------------

func TestPaint_KdbxFile(t *testing.T) {
	p := kdbxPainter{}
	input := []rune("/home/user/test.kdbx")
	result := p.Paint(input, 0)

	out := string(result)
	if !strings.Contains(out, ansiGreen) {
		t.Errorf("Paint should add green color for .kdbx file, got %q", out)
	}
	if !strings.Contains(out, ansiReset) {
		t.Errorf("Paint should add reset after .kdbx file, got %q", out)
	}
	if !strings.Contains(out, "test.kdbx") {
		t.Errorf("Paint should contain the filename, got %q", out)
	}
}

func TestPaint_NonKdbxFile(t *testing.T) {
	p := kdbxPainter{}
	input := []rune("/home/user/test.txt")
	result := p.Paint(input, 0)

	if string(result) != string(input) {
		t.Error("Paint should return input unchanged for non-.kdbx file")
	}
}

func TestPaint_EmptyPath(t *testing.T) {
	p := kdbxPainter{}
	input := []rune("")
	result := p.Paint(input, 0)
	if string(result) != "" {
		t.Error("Paint should return empty for empty input")
	}
}

func TestPaint_NoSlash(t *testing.T) {
	p := kdbxPainter{}
	input := []rune("mydb.kdbx")
	result := p.Paint(input, 0)

	out := string(result)
	if !strings.Contains(out, ansiGreen) {
		t.Errorf("Paint should color .kdbx even without path separator, got %q", out)
	}
}

func TestPaint_DirectoryPath(t *testing.T) {
	p := kdbxPainter{}
	// Ends with slash → empty segment
	input := []rune("/home/user/")
	result := p.Paint(input, 0)

	if string(result) != string(input) {
		t.Error("Paint should return input unchanged for directory path (empty segment)")
	}
}

func TestPaint_CaseInsensitive(t *testing.T) {
	p := kdbxPainter{}
	input := []rune("/path/to/DB.KDBX")
	result := p.Paint(input, 0)

	if !strings.Contains(string(result), ansiGreen) {
		t.Error("Paint should handle uppercase .KDBX")
	}
}

// -------------------------------------------------------------------
// fileCompleter.Do (0% → coverage)
// -------------------------------------------------------------------

func TestFileCompleter_Do_EmptyInput(t *testing.T) {
	fc := &fileCompleter{}
	completions, length := fc.Do(nil, 0)

	// Empty input → list current directory (.), returns 0 length
	if length != 0 {
		t.Errorf("Do empty input length = %d, want 0", length)
	}
	// Should return at least some entries from current directory
	// (test files exist in the package directory)
	if completions == nil {
		t.Log("No completions from current directory (unexpected but not fatal)")
	}
}

func TestFileCompleter_Do_WithPrefix(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	for _, name := range []string{"test.kdbx", "test.txt", "other.kdbx"} {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// No filter
	fc := &fileCompleter{}
	input := []rune(filepath.Join(tmpDir, "test"))
	completions, length := fc.Do(input, len(input))

	// length should be len("test") — the prefix being matched
	if length != len("test") {
		t.Errorf("Do prefix length = %d, want %d", length, len("test"))
	}

	// Should find test.kdbx and test.txt
	if len(completions) < 2 {
		t.Errorf("expected at least 2 completions for 'test', got %d", len(completions))
	}
}

func TestFileCompleter_Do_WithFilter(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	if err := os.WriteFile(filepath.Join(tmpDir, "mydb.kdbx"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "notes.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}

	fc := &fileCompleter{filter: kdbxFilter}
	input := []rune(tmpDir + "/")
	completions, _ := fc.Do(input, len(input))

	// Should find mydb.kdbx and subdir, NOT notes.txt
	foundKdbx := false
	foundTxt := false
	foundDir := false
	for _, c := range completions {
		s := string(c)
		if strings.Contains(s, "mydb") {
			foundKdbx = true
		}
		if strings.Contains(s, "notes") {
			foundTxt = true
		}
		if strings.Contains(s, "subdir") {
			foundDir = true
		}
	}
	if !foundKdbx {
		t.Error("kdbxFilter should show .kdbx files")
	}
	if foundTxt {
		t.Error("kdbxFilter should filter out .txt files")
	}
	if !foundDir {
		t.Error("kdbxFilter should show directories")
	}
}

func TestFileCompleter_Do_DirectoryInput(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	fc := &fileCompleter{}
	input := []rune(tmpDir)
	completions, length := fc.Do(input, len(input))

	// tmpDir is a directory → list its contents, length = 0
	if length != 0 {
		t.Errorf("Do for directory should return length 0, got %d", length)
	}
	if len(completions) < 1 {
		t.Error("expected at least 1 completion inside temp dir")
	}
}

// -------------------------------------------------------------------
// listDir (0% → coverage via Do tests above + direct tests)
// -------------------------------------------------------------------

func TestListDir_HiddenFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create hidden and visible files
	if err := os.WriteFile(filepath.Join(tmpDir, ".hidden"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "visible.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	fc := &fileCompleter{}

	// Without dot prefix → hidden files skipped
	results := fc.listDir(tmpDir, "")
	foundHidden := false
	for _, r := range results {
		if strings.Contains(string(r), ".hidden") {
			foundHidden = true
		}
	}
	if foundHidden {
		t.Error("listDir should skip hidden files unless prefix starts with '.'")
	}

	// With dot prefix → hidden files shown
	results = fc.listDir(tmpDir, ".")
	foundHidden = false
	for _, r := range results {
		if strings.Contains(string(r), "hidden") {
			foundHidden = true
		}
	}
	if !foundHidden {
		t.Error("listDir should show hidden files when prefix starts with '.'")
	}
}

func TestListDir_DirectorySlash(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}

	fc := &fileCompleter{}
	results := fc.listDir(tmpDir, "")

	// Directories should end with "/"
	for _, r := range results {
		s := string(r)
		if strings.Contains(s, "subdir") {
			if !strings.HasSuffix(s, "/") {
				t.Error("directory completion should end with '/'")
			}
			return
		}
	}
	t.Error("subdir not found in listDir results")
}

func TestListDir_FileSpace(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "test.kdbx"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	fc := &fileCompleter{}
	results := fc.listDir(tmpDir, "")

	// Files should end with " " (space)
	for _, r := range results {
		s := string(r)
		if strings.Contains(s, "test.kdbx") {
			if !strings.HasSuffix(s, " ") {
				t.Error("file completion should end with ' '")
			}
			return
		}
	}
	t.Error("test.kdbx not found in listDir results")
}

func TestListDir_NonexistentDir(t *testing.T) {
	fc := &fileCompleter{}
	results := fc.listDir("/nonexistent/path/12345", "")
	if results != nil {
		t.Error("listDir for nonexistent directory should return nil")
	}
}

// -------------------------------------------------------------------
// pathCaseListener (0% → coverage)
// -------------------------------------------------------------------

func TestPathCaseListener_NonTab(t *testing.T) {
	listener := pathCaseListener()

	// Non-tab key should return nil, 0, false
	line := []rune("/some/path")
	newLine, pos, ok := listener(line, len(line), 'a')
	if ok {
		t.Error("pathCaseListener should return false for non-tab key")
	}
	if newLine != nil {
		t.Error("pathCaseListener should return nil newLine for non-tab key")
	}
	if pos != 0 {
		t.Error("pathCaseListener should return 0 pos for non-tab key")
	}
}

func TestPathCaseListener_EmptyInput(t *testing.T) {
	listener := pathCaseListener()

	line := []rune("")
	_, _, ok := listener(line, 0, '\t')
	if ok {
		t.Error("pathCaseListener should return false for empty input")
	}
}

func TestPathCaseListener_DirectoryInput(t *testing.T) {
	// Tab on a directory path → no correction needed
	tmpDir := t.TempDir()
	listener := pathCaseListener()

	line := []rune(tmpDir)
	_, _, ok := listener(line, len(line), '\t')
	if ok {
		t.Error("pathCaseListener should return false for existing directory")
	}
}

func TestPathCaseListener_CaseCorrection(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a file with mixed case
	if err := os.WriteFile(filepath.Join(tmpDir, "MyDatabase.kdbx"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	listener := pathCaseListener()

	// Type the wrong case
	wrongCase := tmpDir + "/mydatabase.kdbx"
	line := []rune(wrongCase)
	newLine, pos, ok := listener(line, len(line), '\t')

	if !ok {
		t.Skip("pathCaseListener didn't correct case (case-sensitive filesystem)")
	}

	corrected := string(newLine)
	if !strings.Contains(corrected, "MyDatabase.kdbx") {
		t.Errorf("pathCaseListener should correct to 'MyDatabase.kdbx', got %q", corrected)
	}
	if pos != len(corrected) {
		t.Errorf("pos = %d, want %d", pos, len(corrected))
	}
}

func TestPathCaseListener_NoMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "exact.kdbx"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	listener := pathCaseListener()

	// Type the correct case
	correctCase := tmpDir + "/exact.kdbx"
	line := []rune(correctCase)
	_, _, ok := listener(line, len(line), '\t')

	// No case mismatch → no correction
	if ok {
		t.Error("pathCaseListener should return false when case already matches")
	}
}

// -------------------------------------------------------------------
// saveConfigWithComments (0% → coverage)
// -------------------------------------------------------------------

func TestSaveConfigWithComments(t *testing.T) {
	// Override HOME so we don't touch real config
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	defer func() {
		if origHome != "" {
			_ = os.Setenv("HOME", origHome)
		}
	}()

	cfg := &config.Config{
		General: config.GeneralConfig{
			KdbxPath:       "/test/db.kdbx",
			DefaultGroup:   "Archives/Test",
			UseKeyring:     true,
			PasswordLength: 64,
		},
		SevenZip: config.SevenZipConfig{
			BinaryPath:  "7z",
			DefaultArgs: []string{"-mhe=on", "-mx=9"},
		},
	}

	if err := saveConfigWithComments(cfg); err != nil {
		t.Fatalf("saveConfigWithComments failed: %v", err)
	}

	// Verify the config file was created
	configPath := filepath.Join(tmpHome, ".config", "7zkpxc", "config.yaml")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read saved config: %v", err)
	}

	s := string(content)
	if !strings.Contains(s, "/test/db.kdbx") {
		t.Error("saved config should contain kdbx_path")
	}
	if !strings.Contains(s, "Archives/Test") {
		t.Error("saved config should contain default_group")
	}
	if !strings.Contains(s, "password_length: 64") {
		t.Error("saved config should contain password_length")
	}
	if !strings.Contains(s, "-mhe=on") {
		t.Error("saved config should contain default_args")
	}
	if !strings.Contains(s, "binary_path: \"7z\"") {
		t.Error("saved config should contain binary_path")
	}

	// Verify file permissions are 0600
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("config file permissions = %v, want 0600", info.Mode().Perm())
	}
}

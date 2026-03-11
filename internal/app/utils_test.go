package app

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// MockPasswordProvider for testing
type MockPasswordProvider struct {
	passwords  map[string][]byte
	attributes map[string]map[string]string // entryPath -> attribute -> value
	calls      []string
}

func NewMockPasswordProvider() *MockPasswordProvider {
	return &MockPasswordProvider{
		passwords:  make(map[string][]byte),
		attributes: make(map[string]map[string]string),
		calls:      make([]string, 0),
	}
}

func (m *MockPasswordProvider) GetPassword(key string) ([]byte, error) {
	m.calls = append(m.calls, key)
	if pass, ok := m.passwords[key]; ok {
		return pass, nil
	}
	return nil, errors.New("not found")
}

func (m *MockPasswordProvider) SetPassword(key string, password []byte) {
	m.passwords[key] = password
}

func (m *MockPasswordProvider) SetAttribute(entryPath, attribute, value string) {
	if m.attributes[entryPath] == nil {
		m.attributes[entryPath] = make(map[string]string)
	}
	m.attributes[entryPath][attribute] = value
}

func (m *MockPasswordProvider) GetAttribute(entryPath, attribute string) (string, error) {
	m.calls = append(m.calls, "attr:"+entryPath+":"+attribute)
	if attrs, ok := m.attributes[entryPath]; ok {
		if val, ok := attrs[attribute]; ok {
			return val, nil
		}
	}
	return "", errors.New("attribute not found")
}

func (m *MockPasswordProvider) Search(query string) ([]string, error) {
	m.calls = append(m.calls, "search:"+query)
	var results []string
	for k := range m.passwords {
		// simple substring match to emulate keepassxc search
		if strings.Contains(k, query) {
			results = append(results, k)
		}
	}
	return results, nil
}

func (m *MockPasswordProvider) UpdateEntryUsername(entryPath, username string) error {
	m.calls = append(m.calls, "update-username:"+entryPath+":"+username)
	if m.attributes[entryPath] == nil {
		m.attributes[entryPath] = make(map[string]string)
	}
	m.attributes[entryPath]["Username"] = username
	return nil
}

func (m *MockPasswordProvider) GetCalls() []string {
	return m.calls
}

// -------------------------------------------------------------------
// updatePathIfMoved
// -------------------------------------------------------------------

func TestUpdatePathIfMoved_SamePath_NoUpdate(t *testing.T) {
	mock := NewMockPasswordProvider()
	entryPath := "backups/test.7z (deadbeef)"
	mock.SetAttribute(entryPath, "Username", "/home/user/test.7z")

	updatePathIfMoved(mock, entryPath, "/home/user/test.7z")

	// UpdateEntryUsername must NOT have been called
	for _, c := range mock.GetCalls() {
		if strings.HasPrefix(c, "update-username:") {
			t.Errorf("updatePathIfMoved called UpdateEntryUsername when path was unchanged: %s", c)
		}
	}
}

func TestUpdatePathIfMoved_PathChanged_UpdatesCalled(t *testing.T) {
	mock := NewMockPasswordProvider()
	entryPath := "backups/test.7z (deadbeef)"
	mock.SetAttribute(entryPath, "Username", "/old/path/test.7z")

	updatePathIfMoved(mock, entryPath, "/new/path/test.7z")

	// UpdateEntryUsername must have been called with the new path
	expected := "update-username:" + entryPath + ":/new/path/test.7z"
	found := false
	for _, c := range mock.GetCalls() {
		if c == expected {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected call %q not found in calls: %v", expected, mock.GetCalls())
	}
	// Verify the attribute was actually updated in the mock
	val, _ := mock.GetAttribute(entryPath, "Username")
	if val != "/new/path/test.7z" {
		t.Errorf("Username after update = %q, want %q", val, "/new/path/test.7z")
	}
}

func TestUpdatePathIfMoved_NoUsername_NoUpdate(t *testing.T) {
	mock := NewMockPasswordProvider()
	entryPath := "backups/test.7z (deadbeef)"
	// Username NOT set → GetAttribute returns error → updatePathIfMoved should be a no-op

	updatePathIfMoved(mock, entryPath, "/any/path/test.7z")

	for _, c := range mock.GetCalls() {
		if strings.HasPrefix(c, "update-username:") {
			t.Errorf("updatePathIfMoved called UpdateEntryUsername when GetAttribute failed: %s", c)
		}
	}
}

// -------------------------------------------------------------------
// UUID helpers
// -------------------------------------------------------------------

func TestGenerateUUID8(t *testing.T) {
	a, err := generateUUID8()
	if err != nil {
		t.Fatalf("generateUUID8 error: %v", err)
	}
	if len(a) != 8 {
		t.Errorf("expected 8 chars, got %d: %q", len(a), a)
	}
	for _, c := range a {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Errorf("non-hex character %q in UUID %q", c, a)
		}
	}
	// Should be random — very unlikely to collide
	b, _ := generateUUID8()
	if a == b {
		t.Errorf("two generateUUID8 calls returned the same value %q (astronomically unlikely)", a)
	}
}

func TestGenerateUniqueUUID8(t *testing.T) {
	mock := NewMockPasswordProvider()
	// Normal case: no existing entries → should succeed on first try
	uuid8, err := generateUniqueUUID8(mock, "backups", "test.7z")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(uuid8) != 8 {
		t.Errorf("expected 8-char uuid8, got %q (len=%d)", uuid8, len(uuid8))
	}
	for _, c := range uuid8 {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Errorf("non-hex character %q in uuid8 %q", c, uuid8)
		}
	}
}

func TestGenerateUniqueUUID8_SkipsCollision(t *testing.T) {
	// This test injects a collision by pre-populating the mock with every
	// possible UUID8 except one, which is not feasible (4B entries), so
	// instead we verify the contract indirectly: after adding a UUID-format
	// entry, generateUniqueUUID8 must return a title that does NOT already exist.
	mock := NewMockPasswordProvider()
	// Add a known entry
	knownUUID := "deadbeef"
	addUUIDEntry(mock, "backups", "test.7z", knownUUID, "/some/test.7z", []byte("pw"))

	uuid8, err := generateUniqueUUID8(mock, "backups", "test.7z")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(uuid8) != 8 {
		t.Errorf("expected 8-char uuid8, got %q", uuid8)
	}
	// The result must not already exist as a full entry path
	title := makeEntryTitle("test.7z", uuid8)
	entryPath := "backups/" + title
	results, _ := mock.Search(title)
	for _, r := range results {
		if r == entryPath {
			t.Errorf("generateUniqueUUID8 returned a UUID (%q) that already exists in the database", uuid8)
		}
	}
}

func TestMakeEntryTitle(t *testing.T) {
	title := makeEntryTitle("backup.7z", "a3b2c1d0")
	if title != "backup.7z (a3b2c1d0)" {
		t.Errorf("got %q, want %q", title, "backup.7z (a3b2c1d0)")
	}
}

func TestParseEntryTitle(t *testing.T) {
	tests := []struct {
		input        string
		wantBasename string
		wantUUID     string
		wantOK       bool
	}{
		{"backup.7z (a3b2c1d0)", "backup.7z", "a3b2c1d0", true},
		{"my archive.tar.gz (ffffffff)", "my archive.tar.gz", "ffffffff", true},
		{"backup.7z", "", "", false},                  // no UUID
		{"backup.7z (ABCD1234)", "", "", false},       // uppercase UUID not accepted
		{"backup.7z (a3b2c1d)", "", "", false},        // 7 chars, not 8
		{"backup.7z (a3b2c1d00)", "", "", false},      // 9 chars
		{"(a3b2c1d0)", "", "", false},                 // empty basename
		{"backup.7z (a3b2c1d0) extra", "", "", false}, // trailing text
	}

	for _, tt := range tests {
		basename, uuid, ok := parseEntryTitle(tt.input)
		if ok != tt.wantOK {
			t.Errorf("parseEntryTitle(%q) ok=%v, want %v", tt.input, ok, tt.wantOK)
			continue
		}
		if ok {
			if basename != tt.wantBasename {
				t.Errorf("parseEntryTitle(%q) basename=%q, want %q", tt.input, basename, tt.wantBasename)
			}
			if uuid != tt.wantUUID {
				t.Errorf("parseEntryTitle(%q) uuid=%q, want %q", tt.input, uuid, tt.wantUUID)
			}
		}
	}
}

func TestMakeParseRoundTrip(t *testing.T) {
	basename := "my.archive.tar.gz"
	uuid8 := "deadbeef"
	title := makeEntryTitle(basename, uuid8)
	gotBasename, gotUUID, ok := parseEntryTitle(title)
	if !ok {
		t.Fatalf("parseEntryTitle(%q) returned ok=false", title)
	}
	if gotBasename != basename {
		t.Errorf("basename round-trip: got %q, want %q", gotBasename, basename)
	}
	if gotUUID != uuid8 {
		t.Errorf("uuid8 round-trip: got %q, want %q", gotUUID, uuid8)
	}
}

// -------------------------------------------------------------------
// normalizeArchiveName
// -------------------------------------------------------------------

func TestNormalizeArchiveName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Standard archives
		{"Standard 7z", "archive.7z", "archive.7z"},
		{"Standard zip", "backup.zip", "backup.zip"},
		{"Standard tar.gz", "data.tar.gz", "data.tar.gz"},

		// Standard split volumes
		{"7z split .001", "archive.7z.001", "archive.7z"},
		{"7z split .002", "archive.7z.002", "archive.7z"},
		{"zip split", "backup.zip.001", "backup.zip"},
		{"tar.gz split", "data.tar.gz.001", "data.tar.gz"},
		{"4-digit split", "file.7z.0001", "file.7z"},

		// RAR part format
		{"RAR part001", "archive.part001.rar", "archive.rar"},
		{"RAR part002", "backup.part002.rar", "backup.rar"},
		{"RAR part100", "data.part100.rar", "data.rar"},

		// RAR old format
		{"RAR .r00", "archive.r00", "archive.rar"},
		{"RAR .r01", "archive.r01", "archive.rar"},
		{"RAR .r99", "archive.r99", "archive.rar"},

		// Case insensitivity
		{"Uppercase 7Z", "ARCHIVE.7Z.001", "ARCHIVE.7Z"},
		{"Mixed case ZIP", "Backup.Zip.001", "Backup.Zip"},
		{"Uppercase RAR", "FILE.PART001.RAR", "FILE.RAR"},

		// Edge cases that should NOT be normalized
		{"Year suffix", "backup.2024", "backup.2024"},
		{"Version number", "app.1.0.exe", "app.1.0.exe"},
		{"Video resolution", "video.1080", "video.1080"},
		{"Decimal number", "data.3.14", "data.3.14"},
		{"Short number", "file.7z.01", "file.7z.01"}, // Only 2 digits

		// Path handling
		{"With path", "/home/user/archive.7z.001", "archive.7z"},
		{"Windows path", "C:\\Users\\test\\backup.zip.001", "backup.zip"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeArchiveName(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeArchiveName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestAnalyzeArchive(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantType  ArchiveType
		wantSplit bool
	}{
		{"Standard archive", "file.7z", ArchiveStandard, false},
		{"Split standard", "file.7z.001", ArchiveSplitStandard, true},
		{"RAR part", "file.part001.rar", ArchiveSplitRarPart, true},
		{"RAR old", "file.r00", ArchiveSplitRarOld, true},
		{"Non-archive", "backup.2024", ArchiveStandard, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := AnalyzeArchive(tt.input)
			if info.Type != tt.wantType {
				t.Errorf("Type = %v, want %v", info.Type, tt.wantType)
			}
			if info.IsSplit != tt.wantSplit {
				t.Errorf("IsSplit = %v, want %v", info.IsSplit, tt.wantSplit)
			}
		})
	}
}

func TestJoinEntry(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		entry    string
		expected string
	}{
		{"Empty prefix", "", "file.7z", "file.7z"},
		{"Simple join", "archives", "file.7z", "archives/file.7z"},
		{"Nested path", "backup/2024", "file.zip", "backup/2024/file.zip"},
		{"Windows separators", "backup\\2024", "file.zip", "backup/2024/file.zip"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := joinEntry(tt.prefix, tt.entry)
			if result != tt.expected {
				t.Errorf("joinEntry(%q, %q) = %q, want %q", tt.prefix, tt.entry, result, tt.expected)
			}
		})
	}
}

// -------------------------------------------------------------------
// GetPasswordForArchive — UUID format (new entries)
// -------------------------------------------------------------------

// addUUIDEntry is a test helper: registers a UUID-format entry in the mock.
func addUUIDEntry(m *MockPasswordProvider, prefix, basename, uuid8, lastKnownPath string, password []byte) string {
	title := makeEntryTitle(basename, uuid8)
	entryPath := title
	if prefix != "" {
		entryPath = prefix + "/" + title
	}
	m.SetPassword(entryPath, password)
	m.SetAttribute(entryPath, "Username", lastKnownPath)
	return entryPath
}

func TestGetPasswordForArchive_UUIDFormat(t *testing.T) {
	mock := NewMockPasswordProvider()
	addUUIDEntry(mock, "backups", "archive.7z", "a3b2c1d0", "/home/user/archive.7z", []byte("secret"))

	pass, _, err := GetPasswordForArchive(mock, "backups", "archive.7z")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(pass) != "secret" {
		t.Errorf("got password %q, want %q", pass, "secret")
	}
}

func TestGetPasswordForArchive_MultiMatch(t *testing.T) {
	mock := NewMockPasswordProvider()
	addUUIDEntry(mock, "backups", "backup.7z", "a3b2c1d0", "/cloud/backup.7z", []byte("pass1"))
	addUUIDEntry(mock, "backups", "backup.7z", "f1e2d3c4", "/local/backup.7z", []byte("pass2"))

	_, _, err := GetPasswordForArchive(mock, "backups", "backup.7z")
	if err == nil {
		t.Fatal("expected MultiMatchError, got nil")
	}
	if !IsMultiMatch(err) {
		t.Fatalf("expected MultiMatchError, got %T: %v", err, err)
	}
	mm := err.(*MultiMatchError)
	if len(mm.Candidates) != 2 {
		t.Errorf("expected 2 candidates, got %d", len(mm.Candidates))
	}
	if mm.Basename != "backup.7z" {
		t.Errorf("Basename = %q, want %q", mm.Basename, "backup.7z")
	}
}

func TestGetPasswordForArchive_UUIDFormat_NoMatch(t *testing.T) {
	mock := NewMockPasswordProvider()
	// Entry for a different basename — should not match
	addUUIDEntry(mock, "backups", "other.7z", "a3b2c1d0", "/home/user/other.7z", []byte("secret"))

	_, _, err := GetPasswordForArchive(mock, "backups", "archive.7z")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !IsPasswordNotFound(err) {
		t.Fatalf("expected PasswordNotFoundError, got %T: %v", err, err)
	}
}

func TestGetPasswordForArchive_UUIDFormat_SplitArchive(t *testing.T) {
	mock := NewMockPasswordProvider()
	// Entry recorded under the normalized name (archive.7z) with UUID
	addUUIDEntry(mock, "backups", "archive.7z", "a3b2c1d0", "/home/user/archive.7z", []byte("split_pass"))

	// User references a split volume
	pass, _, err := GetPasswordForArchive(mock, "backups", "archive.7z.001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(pass) != "split_pass" {
		t.Errorf("got %q, want %q", pass, "split_pass")
	}
}

// -------------------------------------------------------------------
// GetPasswordForArchive — backward compat (old encoded-path format)
// -------------------------------------------------------------------

func TestGetPasswordForArchive_BackwardCompat_Encoded(t *testing.T) {
	mock := NewMockPasswordProvider()
	// Simulate an entry created by the old encodeArchivePath logic
	// Use a relative path so filepath.Abs is predictable in tests
	encoded := encodeArchivePath("/home/user/archive.7z")
	entryPath := "backups/" + encoded
	mock.SetPassword(entryPath, []byte("old_encoded_pass"))
	// Username (last-known-path) enables the backward compat search path
	mock.SetAttribute(entryPath, "Username", "/home/user/archive.7z")

	// Lookup via absolute path — must hit step 2 (backward compat)
	pass, _, err := GetPasswordForArchive(mock, "backups", "/home/user/archive.7z")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(pass) != "old_encoded_pass" {
		t.Errorf("got %q, want %q", pass, "old_encoded_pass")
	}
}

// TestGetPasswordForArchive_BackwardCompat_SearchOldFormat tests the case where
// keepassxc-cli's Search returns an old-format entry (encoded path title) and
// the lookup chain accepts it via Username verification instead of silently
// discarding it. This was the production failure mode after the UUID migration.
func TestGetPasswordForArchive_BackwardCompat_SearchOldFormat(t *testing.T) {
	mock := NewMockPasswordProvider()
	// Old format: title is encoded absolute path, no UUID suffix
	encoded := encodeArchivePath("/home/user/archive.7z")
	entryPath := "backups/" + encoded
	mock.SetPassword(entryPath, []byte("search_old_pass"))
	mock.SetAttribute(entryPath, "Username", "/home/user/archive.7z")

	// Search by basename alone (no absolute path known — simulates post-format scenario)
	pass, _, err := GetPasswordForArchive(mock, "backups", "archive.7z")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(pass) != "search_old_pass" {
		t.Errorf("got %q, want %q", pass, "search_old_pass")
	}
}

func TestGetPasswordForArchive_BackwardCompat_FlatBasename(t *testing.T) {
	mock := NewMockPasswordProvider()
	// Oldest format: just the basename, no encoding
	mock.SetPassword("backups/archive.7z", []byte("flat_pass"))

	pass, _, err := GetPasswordForArchive(mock, "backups", "archive.7z")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(pass) != "flat_pass" {
		t.Errorf("got %q, want %q", pass, "flat_pass")
	}
}

// -------------------------------------------------------------------
// Original test suite (kept for regression)
// -------------------------------------------------------------------

func TestGetPasswordForArchive(t *testing.T) {
	tests := []struct {
		name         string
		archivePath  string
		prefix       string
		setupMock    func(*MockPasswordProvider)
		wantPassword []byte
		wantError    bool
	}{
		{
			name:        "Standard archive - flat basename (backward compat)",
			archivePath: "archive.7z",
			prefix:      "backups",
			setupMock: func(m *MockPasswordProvider) {
				m.SetPassword("backups/archive.7z", []byte("secret123"))
			},
			wantPassword: []byte("secret123"),
			wantError:    false,
		},
		{
			name:        "Split archive - normalized name (flat, backward compat)",
			archivePath: "archive.7z.001",
			prefix:      "backups",
			setupMock: func(m *MockPasswordProvider) {
				m.SetPassword("backups/archive.7z", []byte("split_pass"))
			},
			wantPassword: []byte("split_pass"),
			wantError:    false,
		},
		{
			name:        "Split archive - original name (flat, backward compat)",
			archivePath: "archive.7z.001",
			prefix:      "backups",
			setupMock: func(m *MockPasswordProvider) {
				m.SetPassword("backups/archive.7z.001", []byte("original_pass"))
			},
			wantPassword: []byte("original_pass"),
			wantError:    false,
		},
		{
			name:        "RAR part format (flat, backward compat)",
			archivePath: "backup.part001.rar",
			prefix:      "archives",
			setupMock: func(m *MockPasswordProvider) {
				m.SetPassword("archives/backup.rar", []byte("rar_pass"))
			},
			wantPassword: []byte("rar_pass"),
			wantError:    false,
		},
		{
			name:        "RAR old format (flat, backward compat)",
			archivePath: "data.r00",
			prefix:      "archives",
			setupMock: func(m *MockPasswordProvider) {
				m.SetPassword("archives/data.rar", []byte("old_rar_pass"))
			},
			wantPassword: []byte("old_rar_pass"),
			wantError:    false,
		},
		{
			name:        "Edge case - year suffix",
			archivePath: "backup.2024",
			prefix:      "yearly",
			setupMock: func(m *MockPasswordProvider) {
				m.SetPassword("yearly/backup.2024", []byte("year_pass"))
			},
			wantPassword: []byte("year_pass"),
			wantError:    false,
		},
		{
			name:        "Not found - returns error",
			archivePath: "missing.7z",
			prefix:      "backups",
			setupMock:   func(m *MockPasswordProvider) {},
			wantError:   true,
		},
		{
			name:        "Empty prefix",
			archivePath: "file.zip",
			prefix:      "",
			setupMock: func(m *MockPasswordProvider) {
				m.SetPassword("file.zip", []byte("no_prefix"))
			},
			wantPassword: []byte("no_prefix"),
			wantError:    false,
		},
		{
			name:        "Case insensitive - uppercase split (flat, backward compat)",
			archivePath: "ARCHIVE.7Z.001",
			prefix:      "backups",
			setupMock: func(m *MockPasswordProvider) {
				m.SetPassword("backups/ARCHIVE.7Z", []byte("upper_pass"))
			},
			wantPassword: []byte("upper_pass"),
			wantError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockPasswordProvider()
			tt.setupMock(mock)

			password, _, err := GetPasswordForArchive(mock, tt.prefix, tt.archivePath)

			if tt.wantError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				if !IsPasswordNotFound(err) {
					t.Errorf("Expected PasswordNotFoundError, got %T", err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if string(password) != string(tt.wantPassword) {
					t.Errorf("Password = %q, want %q", password, tt.wantPassword)
				}
			}
		})
	}
}

func TestGetPasswordForArchive_EdgeCases(t *testing.T) {
	t.Run("Nil provider", func(t *testing.T) {
		_, _, err := GetPasswordForArchive(nil, "prefix", "file.7z")
		if err == nil {
			t.Error("Expected error for nil provider")
		}
	})

	t.Run("Empty archive path", func(t *testing.T) {
		mock := NewMockPasswordProvider()
		_, _, err := GetPasswordForArchive(mock, "prefix", "")
		if err == nil {
			t.Error("Expected error for empty archive path")
		}
	})
}

func TestPasswordNotFoundError(t *testing.T) {
	err := &PasswordNotFoundError{
		ArchiveName: "test.7z",
		Tried:       []string{"backups/test.7z", "test.7z"},
	}

	errMsg := err.Error()
	if errMsg == "" {
		t.Error("Error message is empty")
	}

	if !IsPasswordNotFound(err) {
		t.Error("IsPasswordNotFound should return true")
	}

	if IsPasswordNotFound(errors.New("other error")) {
		t.Error("IsPasswordNotFound should return false for other errors")
	}
}

// TestIsPasswordNotFound_WrappedError is the regression test for the bug where
// IsPasswordNotFound used a direct type assertion and failed on wrapped errors.
// resolvePassword in add.go wraps errors before callers check them.
func TestIsPasswordNotFound_WrappedError(t *testing.T) {
	inner := &PasswordNotFoundError{ArchiveName: "archive.7z", Tried: []string{"backups/archive.7z"}}
	wrapped := fmt.Errorf("failed to fetch password: %w", inner)

	if !IsPasswordNotFound(wrapped) {
		t.Error("IsPasswordNotFound should return true for a wrapped PasswordNotFoundError")
	}
}

func TestMultiMatchError(t *testing.T) {
	err := &MultiMatchError{
		Basename: "backup.7z",
		Candidates: []EntryCandidate{
			{EntryPath: "backups/backup.7z (a3b2c1d0)", Title: "backup.7z (a3b2c1d0)", LastKnownPath: "/cloud/backup.7z"},
			{EntryPath: "backups/backup.7z (f1e2d3c4)", Title: "backup.7z (f1e2d3c4)", LastKnownPath: "/local/backup.7z"},
		},
	}

	if !IsMultiMatch(err) {
		t.Error("IsMultiMatch should return true")
	}
	if IsMultiMatch(errors.New("other")) {
		t.Error("IsMultiMatch should return false for other errors")
	}
	if err.Error() == "" {
		t.Error("Error message is empty")
	}
}

// TestIsMultiMatch_WrappedError is the regression test for the bug where
// IsMultiMatch used a direct type assertion and failed on wrapped errors.
func TestIsMultiMatch_WrappedError(t *testing.T) {
	inner := &MultiMatchError{Basename: "backup.7z", Candidates: []EntryCandidate{{}}}
	wrapped := fmt.Errorf("multiple entries: %w", inner)

	if !IsMultiMatch(wrapped) {
		t.Error("IsMultiMatch should return true for a wrapped MultiMatchError")
	}
}

// -------------------------------------------------------------------
// Benchmarks
// -------------------------------------------------------------------

func BenchmarkNormalizeArchiveName(b *testing.B) {
	testCases := []string{
		"archive.7z",
		"archive.7z.001",
		"backup.part001.rar",
		"data.r00",
		"backup.2024",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tc := range testCases {
			normalizeArchiveName(tc)
		}
	}
}

func BenchmarkGetPasswordForArchive(b *testing.B) {
	mock := NewMockPasswordProvider()
	addUUIDEntry(mock, "backups", "archive.7z", "a3b2c1d0", "/home/user/archive.7z", []byte("password"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = GetPasswordForArchive(mock, "backups", "archive.7z.001")
	}
}

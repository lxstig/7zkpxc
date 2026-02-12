package app

import (
	"errors"
	"testing"
)

// MockPasswordProvider for testing
type MockPasswordProvider struct {
	passwords map[string]string
	calls     []string
}

func NewMockPasswordProvider() *MockPasswordProvider {
	return &MockPasswordProvider{
		passwords: make(map[string]string),
		calls:     make([]string, 0),
	}
}

func (m *MockPasswordProvider) GetPassword(key string) (string, error) {
	m.calls = append(m.calls, key)
	if pass, ok := m.passwords[key]; ok {
		return pass, nil
	}
	return "", errors.New("not found")
}

func (m *MockPasswordProvider) SetPassword(key, password string) {
	m.passwords[key] = password
}

func (m *MockPasswordProvider) GetCalls() []string {
	return m.calls
}

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

func TestGetPasswordForArchive(t *testing.T) {
	tests := []struct {
		name         string
		archivePath  string
		prefix       string
		setupMock    func(*MockPasswordProvider)
		wantPassword string
		wantError    bool
		wantCalls    []string
	}{
		{
			name:        "Standard archive - found",
			archivePath: "archive.7z",
			prefix:      "backups",
			setupMock: func(m *MockPasswordProvider) {
				m.SetPassword("backups/archive.7z", "secret123")
			},
			wantPassword: "secret123",
			wantError:    false,
			wantCalls:    []string{"backups/archive.7z"},
		},
		{
			name:        "Split archive - normalized name found",
			archivePath: "archive.7z.001",
			prefix:      "backups",
			setupMock: func(m *MockPasswordProvider) {
				m.SetPassword("backups/archive.7z", "split_pass")
			},
			wantPassword: "split_pass",
			wantError:    false,
			wantCalls:    []string{"backups/archive.7z"},
		},
		{
			name:        "Split archive - original name found",
			archivePath: "archive.7z.001",
			prefix:      "backups",
			setupMock: func(m *MockPasswordProvider) {
				m.SetPassword("backups/archive.7z.001", "original_pass")
			},
			wantPassword: "original_pass",
			wantError:    false,
			wantCalls:    []string{"backups/archive.7z", "backups/archive.7z.001"},
		},
		{
			name:        "RAR part format",
			archivePath: "backup.part001.rar",
			prefix:      "archives",
			setupMock: func(m *MockPasswordProvider) {
				m.SetPassword("archives/backup.rar", "rar_pass")
			},
			wantPassword: "rar_pass",
			wantError:    false,
			wantCalls:    []string{"archives/backup.rar"},
		},
		{
			name:        "RAR old format",
			archivePath: "data.r00",
			prefix:      "archives",
			setupMock: func(m *MockPasswordProvider) {
				m.SetPassword("archives/data.rar", "old_rar_pass")
			},
			wantPassword: "old_rar_pass",
			wantError:    false,
			wantCalls:    []string{"archives/data.rar"},
		},
		{
			name:        "Edge case - year suffix fallback",
			archivePath: "backup.2024",
			prefix:      "yearly",
			setupMock: func(m *MockPasswordProvider) {
				m.SetPassword("yearly/backup.2024", "year_pass")
			},
			wantPassword: "year_pass",
			wantError:    false,
			wantCalls:    []string{"yearly/backup.2024"},
		},
		{
			name:        "Split archive - base name fallback",
			archivePath: "archive.7z.001",
			prefix:      "backups",
			setupMock: func(m *MockPasswordProvider) {
				m.SetPassword("backups/archive", "base_pass")
			},
			wantPassword: "base_pass",
			wantError:    false,
			wantCalls:    []string{"backups/archive.7z", "backups/archive.7z.001", "backups/archive"},
		},
		{
			name:        "Not found - returns error",
			archivePath: "missing.7z",
			prefix:      "backups",
			setupMock:   func(m *MockPasswordProvider) {},
			wantError:   true,
			wantCalls:   []string{"backups/missing.7z"},
		},
		{
			name:        "Empty prefix",
			archivePath: "file.zip",
			prefix:      "",
			setupMock: func(m *MockPasswordProvider) {
				m.SetPassword("file.zip", "no_prefix")
			},
			wantPassword: "no_prefix",
			wantError:    false,
			wantCalls:    []string{"file.zip"},
		},
		{
			name:        "Case insensitive - uppercase",
			archivePath: "ARCHIVE.7Z.001",
			prefix:      "backups",
			setupMock: func(m *MockPasswordProvider) {
				m.SetPassword("backups/ARCHIVE.7Z", "upper_pass")
			},
			wantPassword: "upper_pass",
			wantError:    false,
			wantCalls:    []string{"backups/ARCHIVE.7Z"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockPasswordProvider()
			tt.setupMock(mock)

			password, err := GetPasswordForArchive(mock, tt.prefix, tt.archivePath)

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
				if password != tt.wantPassword {
					t.Errorf("Password = %q, want %q", password, tt.wantPassword)
				}
			}

			calls := mock.GetCalls()
			if len(calls) != len(tt.wantCalls) {
				t.Errorf("Call count = %d, want %d", len(calls), len(tt.wantCalls))
			}
			for i, call := range calls {
				if i < len(tt.wantCalls) && call != tt.wantCalls[i] {
					t.Errorf("Call %d = %q, want %q", i, call, tt.wantCalls[i])
				}
			}
		})
	}
}

func TestGetPasswordForArchive_EdgeCases(t *testing.T) {
	t.Run("Nil provider", func(t *testing.T) {
		_, err := GetPasswordForArchive(nil, "prefix", "file.7z")
		if err == nil {
			t.Error("Expected error for nil provider")
		}
	})

	t.Run("Empty archive path", func(t *testing.T) {
		mock := NewMockPasswordProvider()
		_, err := GetPasswordForArchive(mock, "prefix", "")
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

// Benchmark tests
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
	mock.SetPassword("backups/archive.7z", "password")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = GetPasswordForArchive(mock, "backups", "archive.7z.001")
	}
}

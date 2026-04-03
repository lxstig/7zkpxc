package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lxstig/7zkpxc/internal/config"
	"github.com/lxstig/7zkpxc/internal/keepass"
	"github.com/spf13/cobra"
)

const testMasterPW = "IntegTestP@ss123!"

// setupIntegrationEnv creates a real KDBX database, a valid config pointing
// to it, and returns (tmpDir, dbPath, *keepass.Client). Skips if binaries
// are missing.
func setupIntegrationEnv(t *testing.T) (string, string, *keepass.Client) {
	t.Helper()

	for _, bin := range []string{"keepassxc-cli", "7z"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s not installed", bin)
		}
	}

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.kdbx")

	// Create KDBX
	cmd := exec.Command("keepassxc-cli", "db-create", "-p", dbPath)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	cmd.Stdin = strings.NewReader(testMasterPW + "\n" + testMasterPW + "\n")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("db-create: %v\n%s", err, out)
	}

	// Write config
	t.Setenv("HOME", tmpDir)
	cfgDir := filepath.Join(tmpDir, ".config", "7zkpxc")
	_ = os.MkdirAll(cfgDir, 0755)
	cfgContent := "general:\n  kdbx_path: " + dbPath + "\n  default_group: TestArchives\n  password_length: 64\nsevenzip:\n  binary_path: \"7z\"\n  default_args:\n    - \"-mhe=on\"\n"
	_ = os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(cfgContent), 0644)
	config.ClearCache()

	// Set test hook using unexported package options to authenticate runX commands
	testClientOptions = []keepass.ClientOption{keepass.WithPassword([]byte(testMasterPW))}
	t.Cleanup(func() {
		testClientOptions = nil
		config.ClearCache()
	})

	kp := keepass.New(dbPath, testClientOptions...)
	return tmpDir, dbPath, kp
}

// createTestArchive creates a simple encrypted 7z archive with the given password.
func createTestArchive(t *testing.T, dir, name string, password []byte) string {
	t.Helper()

	// Create a source file to archive
	srcFile := filepath.Join(dir, "testdata.txt")
	_ = os.WriteFile(srcFile, []byte("hello from integration test"), 0644)

	archivePath := filepath.Join(dir, name)
	cmd := exec.Command("7z", "a", "-p"+string(password), "-mhe=on", archivePath, srcFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("7z create failed: %v\n%s", err, out)
	}

	return archivePath
}

// ═══════════════════════════════════════════════════════════════════
//  withKeePassArchive — full pipeline (0% → coverage)
// ═══════════════════════════════════════════════════════════════════

func TestIntegration_WithKeePassArchive_ArchiveNotFound(t *testing.T) {
	setupIntegrationEnv(t)

	err := withKeePassArchive("/nonexistent/archive.7z", false, func(cfg *config.Config, kp *keepass.Client, password []byte, entryPath string) error {
		return nil
	})
	if err == nil {
		t.Fatal("expected error for missing archive")
	}
	if !strings.Contains(err.Error(), "no such file") {
		t.Errorf("expected 'no such file', got: %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════
//  runAdd — full create pipeline
// ═══════════════════════════════════════════════════════════════════

func TestIntegration_RunAdd_CreateArchive(t *testing.T) {
	tmpDir, _, _ := setupIntegrationEnv(t)

	// Create source files
	for _, name := range []string{"file1.txt", "file2.txt"} {
		_ = os.WriteFile(filepath.Join(tmpDir, name), []byte("data: "+name), 0644)
	}

	archiveName := filepath.Join(tmpDir, "newarchive.7z")

	cmd := &cobra.Command{RunE: runAdd}
	cmd.Flags().Bool("fast", false, "")
	cmd.Flags().Bool("best", false, "")
	cmd.Flags().String("volume", "", "")
	cmd.SilenceUsage = true

	err := runAdd(cmd, []string{archiveName, filepath.Join(tmpDir, "file1.txt"), filepath.Join(tmpDir, "file2.txt")})
	if err != nil {
		t.Fatalf("runAdd failed: %v", err)
	}

	// Verify archive was created
	if _, err := os.Stat(archiveName); err != nil {
		t.Errorf("archive should exist: %v", err)
	}
}

func TestIntegration_RunAdd_Existing_UpdatesArchive(t *testing.T) {
	tmpDir, dbPath, _ := setupIntegrationEnv(t)

	// Pre-create an archive and register it in KeePassXC manually
	srcFile := filepath.Join(tmpDir, "original.txt")
	_ = os.WriteFile(srcFile, []byte("original"), 0644)
	archivePath := filepath.Join(tmpDir, "existing.7z")

	// Create via 7z directly with a known password
	archPW := []byte("known_password_42!")
	cmd2 := exec.Command("7z", "a", "-p"+string(archPW), "-mhe=on", archivePath, srcFile)
	if out, err := cmd2.CombinedOutput(); err != nil {
		t.Fatalf("7z create: %v\n%s", err, out)
	}

	// Register in KDBX
	registerEntry(t, dbPath, "TestArchives/existing.7z (aabbccdd)", archivePath, archPW)

	// Now add a new file to the existing archive via runAdd
	newFile := filepath.Join(tmpDir, "extra.txt")
	_ = os.WriteFile(newFile, []byte("extra data"), 0644)

	acmd := &cobra.Command{RunE: runAdd}
	acmd.Flags().Bool("fast", false, "")
	acmd.Flags().Bool("best", false, "")
	acmd.Flags().String("volume", "", "")

	err := runAdd(acmd, []string{archivePath, newFile})
	if err != nil {
		t.Fatalf("runAdd update failed: %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════
//  runExtract / runList / runTest — full pipeline
// ═══════════════════════════════════════════════════════════════════

func TestIntegration_RunExtract(t *testing.T) {
	tmpDir, dbPath, _ := setupIntegrationEnv(t)

	archPW := []byte("extract_test_pw!")
	archPath := createTestArchive(t, tmpDir, "toextract.7z", archPW)

	// Register in KDBX
	registerEntry(t, dbPath, "TestArchives/toextract.7z (11111111)", archPath, archPW)

	extractDir := filepath.Join(tmpDir, "extracted")
	_ = os.MkdirAll(extractDir, 0755)

	cmd := &cobra.Command{RunE: runExtract}
	cmd.Flags().StringP("output", "o", "", "")
	_ = cmd.Flags().Set("output", extractDir)

	err := runExtract(cmd, []string{archPath})
	if err != nil {
		t.Fatalf("runExtract: %v", err)
	}

	// Verify extracted file exists
	if _, err := os.Stat(filepath.Join(extractDir, "testdata.txt")); err != nil {
		t.Errorf("extracted file missing: %v", err)
	}
}

func TestIntegration_RunList(t *testing.T) {
	tmpDir, dbPath, _ := setupIntegrationEnv(t)

	archPW := []byte("list_test_pw!")
	archPath := createTestArchive(t, tmpDir, "tolist.7z", archPW)
	registerEntry(t, dbPath, "TestArchives/tolist.7z (22222222)", archPath, archPW)

	cmd := &cobra.Command{RunE: runList}
	err := runList(cmd, []string{archPath})
	if err != nil {
		t.Fatalf("runList: %v", err)
	}
}

func TestIntegration_RunTest(t *testing.T) {
	tmpDir, dbPath, _ := setupIntegrationEnv(t)

	archPW := []byte("test_cmd_pw!")
	archPath := createTestArchive(t, tmpDir, "totest.7z", archPW)
	registerEntry(t, dbPath, "TestArchives/totest.7z (33333333)", archPath, archPW)

	cmd := &cobra.Command{RunE: runTest}
	err := runTest(cmd, []string{archPath})
	if err != nil {
		t.Fatalf("runTest: %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════
//  runMv — full pipeline
// ═══════════════════════════════════════════════════════════════════

func TestIntegration_RunMv(t *testing.T) {
	tmpDir, dbPath, _ := setupIntegrationEnv(t)

	archPW := []byte("mv_test_pw!")
	archPath := createTestArchive(t, tmpDir, "tomove.7z", archPW)
	registerEntry(t, dbPath, "TestArchives/tomove.7z (44444444)", archPath, archPW)

	newPath := filepath.Join(tmpDir, "moved.7z")

	cmd := &cobra.Command{RunE: runMv}
	cmd.Flags().Bool("no-verify", false, "")

	err := runMv(cmd, []string{archPath, newPath})
	if err != nil {
		t.Fatalf("runMv: %v", err)
	}

	// Old gone, new exists
	if _, err := os.Stat(archPath); !os.IsNotExist(err) {
		t.Error("old archive should be gone")
	}
	if _, err := os.Stat(newPath); err != nil {
		t.Errorf("new archive should exist: %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════
//  runRelink — full pipeline
// ═══════════════════════════════════════════════════════════════════

func TestIntegration_RunRelink_SingleArchive(t *testing.T) {
	tmpDir, dbPath, kp := setupIntegrationEnv(t)

	archPW := []byte("relink_test_pw!")
	archPath := createTestArchive(t, tmpDir, "torelink.7z", archPW)
	entryPath := "TestArchives/torelink.7z (77777777)"
	registerEntry(t, dbPath, entryPath, archPath, archPW)

	// Rename the archive so its KeePassXC path breaks natively
	renamedPath := filepath.Join(tmpDir, "renamed_relink.7z")
	if err := os.Rename(archPath, renamedPath); err != nil {
		t.Fatalf("rename failed: %v", err)
	}

	cmd := &cobra.Command{RunE: runRelink}
	err := runRelink(cmd, []string{renamedPath})
	if err != nil {
		t.Fatalf("runRelink: %v", err)
	}

	// Verify KeePassXC uses the new name/path
	username, _ := kp.GetAttribute("TestArchives/renamed_relink.7z (77777777)", "Username")
	if username != renamedPath {
		t.Errorf("expected KeePassXC Username to be %q, got %q", renamedPath, username)
	}

	if _, err := kp.GetPassword(entryPath); err == nil {
		t.Error("old entry should be renamed/deleted")
	}
}

func TestIntegration_RunRelink_Directory(t *testing.T) {
	tmpDir, dbPath, kp := setupIntegrationEnv(t)

	// Create and register multiple files
	pw1 := []byte("dir_test_pw1!")
	path1 := createTestArchive(t, tmpDir, "dirrelink1.7z", pw1)
	registerEntry(t, dbPath, "TestArchives/dirrelink1.7z (88888888)", path1, pw1)

	pw2 := []byte("dir_test_pw2!")
	path2 := createTestArchive(t, tmpDir, "dirrelink2.7z", pw2)
	registerEntry(t, dbPath, "TestArchives/dirrelink2.7z (99999999)", path2, pw2)

	// Move them to a subfolder
	subDir := filepath.Join(tmpDir, "subfolder")
	_ = os.MkdirAll(subDir, 0755)

	newPath1 := filepath.Join(subDir, "dirrelink1.7z")
	_ = os.Rename(path1, newPath1)

	newPath2 := filepath.Join(subDir, "dirrelink2.7z")
	_ = os.Rename(path2, newPath2)

	// Relink the subDir where the archives actually are!
	// findArchivesInDir uses os.ReadDir which is not recursive.
	cmd := &cobra.Command{RunE: runRelink}
	err := runRelink(cmd, []string{subDir})
	if err != nil {
		t.Fatalf("runRelink directory: %v", err)
	}

	// Check if both were updated natively
	u1, _ := kp.GetAttribute("TestArchives/dirrelink1.7z (88888888)", "Username")
	if u1 != newPath1 {
		t.Errorf("expected %q, got %q", newPath1, u1)
	}
	u2, _ := kp.GetAttribute("TestArchives/dirrelink2.7z (99999999)", "Username")
	if u2 != newPath2 {
		t.Errorf("expected %q, got %q", newPath2, u2)
	}
}

// ═══════════════════════════════════════════════════════════════════
//  runRemove — full pipeline (force mode)
// ═══════════════════════════════════════════════════════════════════

func TestIntegration_RunRemove_Force(t *testing.T) {
	tmpDir, dbPath, _ := setupIntegrationEnv(t)

	archPW := []byte("remove_test_pw!")
	archPath := createTestArchive(t, tmpDir, "toremove.7z", archPW)
	registerEntry(t, dbPath, "TestArchives/toremove.7z (55555555)", archPath, archPW)

	cmd := &cobra.Command{RunE: runRemove}
	cmd.Flags().BoolP("force", "f", false, "")
	_ = cmd.Flags().Set("force", "true")

	err := runRemove(cmd, []string{archPath})
	if err != nil {
		t.Fatalf("runRemove: %v", err)
	}

	// Archive should be deleted
	if _, err := os.Stat(archPath); !os.IsNotExist(err) {
		t.Error("archive should be deleted")
	}
}

// ═══════════════════════════════════════════════════════════════════
//  Helper: register entry in KDBX via keepassxc-cli
// ═══════════════════════════════════════════════════════════════════

func registerEntry(t *testing.T, dbPath, entryPath, archivePath string, password []byte) {
	t.Helper()

	// Need to create group first
	parts := strings.SplitN(entryPath, "/", 2)
	if len(parts) == 2 {
		mkdirCmd := exec.Command("keepassxc-cli", "mkdir", dbPath, parts[0])
		mkdirCmd.Env = append(os.Environ(), "LC_ALL=C")
		mkdirCmd.Stdin = strings.NewReader(testMasterPW + "\n")
		_, _ = mkdirCmd.CombinedOutput() // Ignore error (group may exist)
	}

	cmd := exec.Command("keepassxc-cli", "add", dbPath, entryPath,
		"--username", archivePath, "--url", "https://github.com/lxstig/7zkpxc", "-p")
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	cmd.Stdin = strings.NewReader(testMasterPW + "\n" + string(password) + "\n" + string(password) + "\n")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("registerEntry(%s): %v\n%s", entryPath, err, out)
	}
}

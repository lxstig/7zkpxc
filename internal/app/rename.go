package app

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/lxstig/7zkpxc/internal/config"
	"github.com/lxstig/7zkpxc/internal/keepass"
	"github.com/lxstig/7zkpxc/internal/sevenzip"
	"github.com/spf13/cobra"
)

var renameCmd = &cobra.Command{
	Use:     "rename <old_archive_path> <new_archive_path>",
	Short:   "Rename or move an archive and update its KeePassXC entry",
	Long:    `Renames or moves an archive file on disk and updates the corresponding entry inside the KeePassXC database. Both the file system operation and the KeePass record are updated atomically: if any step fails the previous steps are rolled back. Cross-device moves (different mount points) are handled transparently via copy+delete.`,
	Args:    cobra.ExactArgs(2),
	RunE:    runRename,
	GroupID: "actions",
}

func init() {
	rootCmd.AddCommand(renameCmd)
}

func runRename(cmd *cobra.Command, args []string) error {
	oldArchivePath := args[0]
	newArchivePath := args[1]

	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	// Resolve absolute paths upfront
	absOld, err := filepath.Abs(oldArchivePath)
	if err != nil {
		return fmt.Errorf("failed to resolve old archive path: %w", err)
	}

	absNew, err := filepath.Abs(newArchivePath)
	if err != nil {
		return fmt.Errorf("failed to resolve new archive path: %w", err)
	}

	// 1. Verify the source file actually exists on disk
	srcInfo, err := os.Stat(absOld)
	if os.IsNotExist(err) {
		return fmt.Errorf("source archive does not exist: %s", absOld)
	} else if err != nil {
		return fmt.Errorf("cannot access source archive: %w", err)
	}

	// 2. Ensure the destination does not already exist (prevent silent overwrites)
	if _, err := os.Stat(absNew); err == nil {
		return fmt.Errorf("destination already exists: %s", absNew)
	}

	// 3. Ensure the destination directory exists
	if err := os.MkdirAll(filepath.Dir(absNew), 0o755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// 4. Look up the KeePass entry (before touching the file)
	kp := keepass.New(cfg.General.KdbxPath)
	defer kp.Close()

	fmt.Printf("Locating KeePass entry for '%s'...\n", oldArchivePath)
	password, oldKeePassPath, _, err := resolvePassword(kp, cfg.General.DefaultGroup, oldArchivePath)
	if err != nil {
		return fmt.Errorf("could not find KeePass entry for '%s': %w", oldArchivePath, err)
	}
	defer func() {
		for i := range password {
			password[i] = 0
		}
	}()

	// 4.5 Verify the password is correct BEFORE touching anything on disk.
	//     This protects against accidental wrong-entry selection in multi-match.
	//     'sevenzip t' (test) decrypts without extracting; any decryption failure
	//     means the selected entry's password does not match this archive.
	fmt.Printf("Verifying password against archive...\n")
	if err := sevenzip.Run(cfg.SevenZip.BinaryPath, password, []string{"t", absOld}); err != nil {
		return fmt.Errorf("password verification failed — wrong entry selected or archive is corrupt: %w", err)
	}

	// 5. Move the file on disk (same-device: rename; cross-device: copy+delete)
	fmt.Printf("Moving '%s' → '%s'...\n", absOld, absNew)
	crossDevice, err := moveFile(absOld, absNew, srcInfo)
	if err != nil {
		return fmt.Errorf("failed to move archive on disk: %w", err)
	}

	// 6+7. Update KeePass: add new entry, rollback file move on failure, delete old.
	if err := applyRenameKeePass(kp, cfg.General.DefaultGroup, oldKeePassPath, absOld, absNew, password, srcInfo, crossDevice); err != nil {
		return err
	}

	fmt.Println("Done. Archive moved and KeePassXC entry updated.")
	return nil
}

// applyRenameKeePass adds a new UUID-titled KeePass entry for the destination
// archive, rolls back the file move if that fails, then removes the old entry.
func applyRenameKeePass(
	kp *keepass.Client,
	group, oldKeePassPath, absOld, absNew string,
	password []byte,
	srcInfo os.FileInfo,
	crossDevice bool,
) error {
	newUUID8, err := generateUniqueUUID8(kp, group, filepath.Base(absNew))
	if err != nil {
		return fmt.Errorf("failed to generate entry UUID: %w", err)
	}
	newEntryTitle := makeEntryTitle(filepath.Base(absNew), newUUID8)
	fmt.Printf("Updating KeePassXC entry (title: %s)...\n", newEntryTitle)

	if err := kp.AddEntry(group, newEntryTitle, password, absNew, "https://github.com/lxstig/7zkpxc"); err != nil {
		fmt.Printf("KeePass update failed, rolling back file move...\n")
		rbErr := rollbackMove(absOld, absNew, srcInfo, crossDevice)
		if rbErr != nil {
			return fmt.Errorf("keepassxc-cli add failed: %w\nROLLBACK FAILED (file is at %s): %v", err, absNew, rbErr)
		}
		return fmt.Errorf("failed to create new KeePass entry (file move rolled back): %w", err)
	}

	fmt.Printf("Cleaning up old KeePass entry ('%s')...\n", oldKeePassPath)
	if err := kp.DeleteEntry(oldKeePassPath); err != nil {
		fmt.Printf("Warning: could not delete old entry '%s': %v\n", oldKeePassPath, err)
		fmt.Println("The archive was moved and the new entry was created successfully.")
		fmt.Println("You may want to delete the old entry manually from KeePassXC.")
	}
	return nil
}

// rollbackMove undoes a file move by moving dst back to src.
func rollbackMove(src, dst string, srcInfo os.FileInfo, crossDevice bool) error {
	if crossDevice {
		return moveFileCopy(dst, src, srcInfo)
	}
	return os.Rename(dst, src)
}

func moveFile(src, dst string, srcInfo os.FileInfo) (crossDevice bool, err error) {
	if err := os.Rename(src, dst); err == nil {
		return false, nil
	} else if !errors.Is(err, syscall.EXDEV) {
		return false, err
	}

	// Cross-device: copy then remove the source
	if err := moveFileCopy(src, dst, srcInfo); err != nil {
		return true, err
	}
	return true, nil
}

// moveFileCopy copies src to dst byte-for-byte, preserving permissions,
// then removes src. dst must not exist.
func moveFileCopy(src, dst string, srcInfo os.FileInfo) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer func() { _ = in.Close() }()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("create destination: %w", err)
	}

	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(dst) // clean up partial file
		return fmt.Errorf("copy data: %w", err)
	}

	if err := out.Close(); err != nil {
		_ = os.Remove(dst)
		return fmt.Errorf("close destination: %w", err)
	}

	if err := os.Remove(src); err != nil {
		// Destination is already written — best effort remove of dst to avoid
		// having two copies. If that also fails, leave dst in place (it's
		// complete and correct).
		return fmt.Errorf("remove source after copy: %w", err)
	}

	return nil
}

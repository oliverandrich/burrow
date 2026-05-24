package selfupdate

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"

	"github.com/minio/selfupdate"
)

// install replaces the binary at targetPath with the contents of r.
// When sha256sum is non-nil, minio/selfupdate verifies the stream
// against it before swapping. On checksum mismatch (or any other
// failure) the original binary is preserved via the package's
// atomic-rename + rollback path.
//
// targetPath empty means "the running executable".
func install(r io.Reader, targetPath string, sha256sum []byte) error {
	err := selfupdate.Apply(r, selfupdate.Options{
		TargetPath: targetPath,
		Checksum:   sha256sum,
	})
	if err == nil {
		return nil
	}
	if rollbackErr := selfupdate.RollbackError(err); rollbackErr != nil {
		return fmt.Errorf("selfupdate: apply failed AND rollback failed (%w); original binary may be in inconsistent state", rollbackErr)
	}
	if errors.Is(err, fs.ErrPermission) {
		t := targetPath
		if t == "" {
			t = "the running binary"
		}
		return fmt.Errorf("selfupdate: cannot write %s: permission denied — try running as the binary's owner: %w", t, err)
	}
	return fmt.Errorf("selfupdate: apply: %w", err)
}

// verifySha256 returns nil when the SHA-256 of data matches want.
// Used to verify a downloaded archive against the checksum stored
// in checksums.txt before extracting and installing.
func verifySha256(data, want []byte) error {
	got := sha256.Sum256(data)
	if !bytes.Equal(got[:], want) {
		return fmt.Errorf("selfupdate: archive checksum mismatch (got %x, want %x); refusing to install", got, want)
	}
	return nil
}

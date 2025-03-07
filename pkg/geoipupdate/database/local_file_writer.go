package database

import (
	"crypto/md5"
	"fmt"
	"hash"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"github.com/pkg/errors"
)

const timestampFormat = "20060102T150405Z0700"

//LocalFileDatabaseWriter is a database.Writer that stores the database to the local file system
type LocalFileDatabaseWriter struct {
	filePath      string
	symlinkPath   string
	lockFilePath  string
	verbose       bool
	lock          *flock.Flock
	oldHash       string
	fileWriter    io.Writer
	temporaryFile *os.File
	md5Writer     hash.Hash
	lastModified  time.Time
}

// NewLocalFileDatabaseWriter create a LocalFileDatabaseWriter. It creates the
// necessary lock and temporary files to protect the database from concurrent
// writes.
func NewLocalFileDatabaseWriter(filePath string, lockFilePath string, verbose bool) (*LocalFileDatabaseWriter, error) {
	dbWriter := &LocalFileDatabaseWriter{
		filePath:     filePath,
		symlinkPath:  filePath,
		lockFilePath: lockFilePath,
		verbose:      verbose,
	}

	var err error
	if dbWriter.lock, err = CreateLockFile(lockFilePath, verbose); err != nil {
		return nil, err
	}
	if err = dbWriter.createOldMD5Hash(); err != nil {
		return nil, err
	}

	temporaryFilename := fmt.Sprintf("%s.temporary", dbWriter.filePath)
	dbWriter.temporaryFile, err = os.OpenFile( //nolint:gosec
		temporaryFilename,
		os.O_WRONLY|os.O_CREATE|os.O_TRUNC,
		0644,
	)
	if err != nil {
		return nil, errors.Wrap(err, "error creating temporary file")
	}
	dbWriter.md5Writer = md5.New()
	dbWriter.fileWriter = io.MultiWriter(dbWriter.md5Writer, dbWriter.temporaryFile)

	return dbWriter, nil
}

func (writer *LocalFileDatabaseWriter) createOldMD5Hash() error {
	currentDatabaseFile, err := os.Open(writer.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			writer.oldHash = ZeroMD5
			return nil
		}
		return errors.Wrap(err, "error opening database")
	}

	defer func() {
		err := currentDatabaseFile.Close()
		if err != nil {
			log.Println(errors.Wrap(err, "error closing database"))
		}
	}()
	oldHash := md5.New()
	if _, err := io.Copy(oldHash, currentDatabaseFile); err != nil {
		return errors.Wrap(err, "error calculating database hash")
	}
	writer.oldHash = fmt.Sprintf("%x", oldHash.Sum(nil))
	if writer.verbose {
		log.Printf("Calculated MD5 sum for %s: %s", writer.filePath, writer.oldHash)
	}
	return nil
}

//Write writes data to temporary file
func (writer *LocalFileDatabaseWriter) Write(p []byte) (int, error) {
	return writer.fileWriter.Write(p)
}

//Close closes the temporary file and releases the file lock
func (writer *LocalFileDatabaseWriter) Close() error {
	if err := writer.temporaryFile.Close(); err != nil && errors.Cause(err) == os.ErrClosed {
		return errors.Wrap(err, "error closing temporary file")
	}
	if err := os.Remove(writer.temporaryFile.Name()); err != nil && errors.Cause(err) == os.ErrNotExist {
		return errors.Wrap(err, "error removing temporary file")
	}
	if err := writer.lock.Unlock(); err != nil {
		return errors.Wrap(err, "error releasing lock file")
	}
	return nil
}

// ValidHash checks that the temporary file's MD5 matches the given hash.
func (writer *LocalFileDatabaseWriter) ValidHash(expectedHash string) error {
	actualHash := fmt.Sprintf("%x", writer.md5Writer.Sum(nil))
	if !strings.EqualFold(actualHash, expectedHash) {
		return errors.Errorf("md5 of new database (%s) does not match expected md5 (%s)", actualHash, expectedHash)
	}
	return nil
}

func (writer *LocalFileDatabaseWriter) UpdateFilepath(lastModified *time.Time) {
	if lastModified != nil {
		writer.filePath = fmt.Sprintf("%s-%s", writer.filePath, lastModified.UTC().Format(timestampFormat))
	}
}

// SetFileModificationTime sets the database's file access and modified times
// to the given time.
func (writer *LocalFileDatabaseWriter) SetFileModificationTime(lastModified time.Time) error {
	if err := os.Chtimes(writer.filePath, lastModified, lastModified); err != nil {
		return errors.Wrap(err, "error setting times on file")
	}
	writer.lastModified = lastModified
	return nil
}

// Commit renames the temporary file to the name of the database file and syncs
// the directory.
func (writer *LocalFileDatabaseWriter) Commit() error {
	if err := writer.temporaryFile.Sync(); err != nil {
		return errors.Wrap(err, "error syncing temporary file")
	}
	if err := writer.temporaryFile.Close(); err != nil {
		return errors.Wrap(err, "error closing temporary file")
	}
	if err := os.Rename(writer.temporaryFile.Name(), writer.filePath); err != nil {
		return errors.Wrap(err, "error moving database into place")
	}

	if err := writer.fsyncDir(); err != nil {
		return err
	}

	// Create a symlink to the recently downloaded file
	// target file is expected to be in same directory as the symlink
	if writer.symlinkPath != writer.filePath {
		if err := os.Remove(writer.symlinkPath); err != nil && os.IsExist(err) {
			return errors.Wrap(err, "error removing existing symlink")
		}
		if err := os.Symlink(filepath.Base(writer.filePath), writer.symlinkPath); err != nil {
			return errors.Wrap(err, "error creating symlink to new database file")
		}
		return writer.fsyncDir()
	}
	return nil
}

// fsync the directory. http://austingroupbugs.net/view.php?id=672
func (writer *LocalFileDatabaseWriter) fsyncDir() error {
	dh, err := os.Open(filepath.Dir(writer.filePath))
	if err != nil {
		return errors.Wrap(err, "error opening database directory")
	}
	defer func() {
		if err := dh.Close(); err != nil {
			log.Fatalf("Error closing directory: %+v", errors.Wrap(err, "closing directory"))
		}
	}()

	// We ignore Sync errors as they primarily happen on file systems that do
	// not support sync.
	_ = dh.Sync()
	return nil
}

//GetHash returns the hash of the current database file
func (writer *LocalFileDatabaseWriter) GetHash() string {
	return writer.oldHash
}

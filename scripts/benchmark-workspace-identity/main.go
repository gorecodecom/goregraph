package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) != 2 {
		die(errors.New("usage: benchmark-workspace-identity /absolute/workspace"))
	}
	digest, err := hashWorkspace(os.Args[1])
	if err != nil {
		die(err)
	}
	fmt.Println(digest)
}

func die(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(2)
}

func hashWorkspace(root string) (string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("workspace is not a directory: %s", root)
	}

	treeHash := sha256.New()
	_, _ = treeHash.Write([]byte("goregraph-benchmark-workspace-v1\x00"))
	err = fs.WalkDir(os.DirFS(root), ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == "." {
			return nil
		}
		path = filepath.ToSlash(path)
		switch {
		case entry.IsDir():
			writeTreeRecord(treeHash, 'D', path, nil)
			return nil
		case entry.Type().IsRegular():
			fileDigest, err := hashWorkspaceFile(filepath.Join(root, filepath.FromSlash(path)))
			if err != nil {
				return fmt.Errorf("hash workspace file %s: %w", path, err)
			}
			writeTreeRecord(treeHash, 'F', path, fileDigest)
			return nil
		default:
			return fmt.Errorf("workspace entry is not a regular file or directory: %s", path)
		}
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(treeHash.Sum(nil)), nil
}

func hashWorkspaceFile(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	fileHash := sha256.New()
	_, copyErr := io.Copy(fileHash, file)
	if err := errors.Join(copyErr, file.Close()); err != nil {
		return nil, err
	}
	return fileHash.Sum(nil), nil
}

func writeTreeRecord(target hash.Hash, kind byte, path string, contentDigest []byte) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(path)))
	_, _ = target.Write([]byte{kind})
	_, _ = target.Write(length[:])
	_, _ = target.Write([]byte(path))
	_, _ = target.Write(contentDigest)
}

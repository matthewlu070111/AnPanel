package agent

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/matthewlu070111/anpanel/internal/domain"
)

const (
	maxFileBytes   = 2 << 20   // 2 MiB editor limit
	maxUploadBytes = 200 << 20 // 200 MiB upload limit
)

var fileRoots = []string{"/"}

func safeFilePath(raw string) (string, error) {
	if raw == "" {
		return "/", nil
	}
	p, err := filepath.Abs(raw)
	if err != nil {
		return "", err
	}
	p = filepath.Clean(p)
	ok := false
	for _, root := range fileRoots {
		if p == root {
			ok = true
			break
		}
		rel, e := filepath.Rel(root, p)
		if e == nil && rel != ".." && !strings.HasPrefix(rel, "../") {
			ok = true
			break
		}
	}
	if !ok {
		return "", errors.New("path is outside filesystem root")
	}
	return p, nil
}

func listFiles(path string) ([]domain.FileEntry, error) {
	p, err := safeFilePath(path)
	if err != nil {
		return nil, err
	}
	// If path does not exist yet, return empty under a known root.
	st, err := os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return []domain.FileEntry{}, nil
		}
		return nil, err
	}
	if !st.IsDir() {
		return nil, errors.New("path is not a directory")
	}
	entries, err := os.ReadDir(p)
	if err != nil {
		return nil, err
	}
	out := make([]domain.FileEntry, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, domain.FileEntry{
			Name:    e.Name(),
			Path:    filepath.Join(p, e.Name()),
			IsDir:   e.IsDir(),
			Size:    info.Size(),
			Mode:    info.Mode().String(),
			ModTime: info.ModTime().UTC().Format(time.RFC3339),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].IsDir != out[j].IsDir {
			return out[i].IsDir
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

func readFileContent(path string) (string, error) {
	p, err := safeFilePath(path)
	if err != nil {
		return "", err
	}
	st, err := os.Stat(p)
	if err != nil {
		return "", err
	}
	if st.IsDir() {
		return "", errors.New("path is a directory")
	}
	if st.Size() > maxFileBytes {
		return "", fmt.Errorf("file larger than %d bytes cannot be edited in the panel", maxFileBytes)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	// Reject obvious binary.
	if strings.Contains(string(b), "\x00") {
		return "", errors.New("binary files cannot be edited in the panel")
	}
	return string(b), nil
}

func writeFileContent(path, content string) (ActionResult, error) {
	p, err := safeFilePath(path)
	if err != nil {
		return ActionResult{}, err
	}
	if len(content) > maxFileBytes {
		return ActionResult{}, fmt.Errorf("content exceeds %d bytes", maxFileBytes)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return ActionResult{}, err
	}
	mode := os.FileMode(0644)
	if st, e := os.Stat(p); e == nil {
		if st.IsDir() {
			return ActionResult{}, errors.New("path is a directory")
		}
		mode = st.Mode()
	}
	tmp := p + ".anpanel.tmp"
	if err := os.WriteFile(tmp, []byte(content), mode); err != nil {
		return ActionResult{}, err
	}
	if err := os.Rename(tmp, p); err != nil {
		_ = os.Remove(tmp)
		return ActionResult{}, err
	}
	return ActionResult{Output: "file saved: " + p}, nil
}

func mkdirPath(path string) (ActionResult, error) {
	p, err := safeFilePath(path)
	if err != nil {
		return ActionResult{}, err
	}
	if err := os.MkdirAll(p, 0755); err != nil {
		return ActionResult{}, err
	}
	return ActionResult{Output: "directory created: " + p}, nil
}

func deletePath(path string) (ActionResult, error) {
	p, err := safeFilePath(path)
	if err != nil {
		return ActionResult{}, err
	}
	// Disallow deleting root allowlist entries themselves.
	for _, root := range fileRoots {
		if p == root {
			return ActionResult{}, errors.New("refusing to delete allowed root directory")
		}
	}
	if err := os.RemoveAll(p); err != nil {
		return ActionResult{}, err
	}
	return ActionResult{Output: "deleted: " + p}, nil
}

func renamePath(from, to string) (ActionResult, error) {
	src, err := safeFilePath(from)
	if err != nil {
		return ActionResult{}, err
	}
	dst, err := safeFilePath(to)
	if err != nil {
		return ActionResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return ActionResult{}, err
	}
	if err := os.Rename(src, dst); err != nil {
		// cross-device fallback
		if err2 := copyRemove(src, dst); err2 != nil {
			return ActionResult{}, err
		}
	}
	return ActionResult{Output: "renamed to " + dst}, nil
}

func copyRemove(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	st, err := in.Stat()
	if err != nil {
		return err
	}
	if st.IsDir() {
		return errors.New("cannot rename directories across devices")
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, st.Mode())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Remove(src)
}

func copyPath(from, to string) (ActionResult, error) {
	src, err := safeFilePath(from)
	if err != nil {
		return ActionResult{}, err
	}
	dst, err := safeFilePath(to)
	if err != nil {
		return ActionResult{}, err
	}
	if src == dst {
		return ActionResult{}, errors.New("source and destination are the same")
	}
	// Refuse to copy a directory into itself.
	if rel, e := filepath.Rel(src, dst); e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return ActionResult{}, errors.New("cannot copy a path into itself")
	}
	if _, err := os.Stat(dst); err == nil {
		return ActionResult{}, errors.New("destination already exists")
	} else if !os.IsNotExist(err) {
		return ActionResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return ActionResult{}, err
	}
	if err := copyRecursive(src, dst); err != nil {
		_ = os.RemoveAll(dst)
		return ActionResult{}, err
	}
	return ActionResult{Output: "copied to " + dst}, nil
}

func movePath(from, to string) (ActionResult, error) {
	// Prefer rename; fall back to copy+delete for cross-device moves.
	if _, err := renamePath(from, to); err == nil {
		return ActionResult{Output: "moved to " + to}, nil
	} else {
		if _, err2 := copyPath(from, to); err2 != nil {
			return ActionResult{}, err
		}
		if _, err2 := deletePath(from); err2 != nil {
			return ActionResult{}, fmt.Errorf("copied but failed to remove source: %w", err2)
		}
		return ActionResult{Output: "moved to " + to}, nil
	}
}

func copyRecursive(src, dst string) error {
	st, err := os.Stat(src)
	if err != nil {
		return err
	}
	if st.IsDir() {
		if err := os.MkdirAll(dst, st.Mode().Perm()); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if err := copyRecursive(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, st.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// sanitizeUploadName returns a single path segment filename or an error.
func sanitizeUploadName(name string) (string, error) {
	name = strings.TrimSpace(name)
	name = filepath.Base(filepath.Clean("/" + strings.ReplaceAll(name, "\\", "/")))
	if name == "" || name == "." || name == ".." {
		return "", errors.New("invalid file name")
	}
	if strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return "", errors.New("invalid file name")
	}
	return name, nil
}

// saveUploadedFile streams r into dir/name with optional overwrite. size may be -1 if unknown.
func saveUploadedFile(dir, name string, r io.Reader, size int64, overwrite bool) (string, error) {
	dirPath, err := safeFilePath(dir)
	if err != nil {
		return "", err
	}
	st, err := os.Stat(dirPath)
	if err != nil {
		return "", err
	}
	if !st.IsDir() {
		return "", errors.New("upload path is not a directory")
	}
	name, err = sanitizeUploadName(name)
	if err != nil {
		return "", err
	}
	if size > maxUploadBytes {
		return "", fmt.Errorf("file larger than %d bytes", maxUploadBytes)
	}
	dst := filepath.Join(dirPath, name)
	dst, err = safeFilePath(dst)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(dst); err == nil && !overwrite {
		return "", errors.New("file already exists")
	} else if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	var limited io.Reader
	if size < 0 {
		limited = io.LimitReader(r, maxUploadBytes+1)
	} else {
		limited = io.LimitReader(r, size)
	}
	tmp := dst + ".anpanel.upload.tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return "", err
	}
	written, err := io.Copy(f, limited)
	closeErr := f.Close()
	if err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return "", closeErr
	}
	if size < 0 && written > maxUploadBytes {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("file larger than %d bytes", maxUploadBytes)
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	return dst, nil
}

// openDownload validates path and opens a regular file for streaming download.
func openDownload(path string) (file *os.File, size int64, name string, err error) {
	p, err := safeFilePath(path)
	if err != nil {
		return nil, 0, "", err
	}
	st, err := os.Stat(p)
	if err != nil {
		return nil, 0, "", err
	}
	if st.IsDir() {
		return nil, 0, "", errors.New("path is a directory")
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, 0, "", err
	}
	return f, st.Size(), filepath.Base(p), nil
}

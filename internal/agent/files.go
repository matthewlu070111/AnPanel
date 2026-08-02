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

const maxFileBytes = 2 << 20 // 2 MiB editor limit

var fileRoots = []string{
	"/var/www",
	"/www",
	"/srv",
	"/opt",
	"/home",
	"/etc/anpanel/compose",
	"/var/lib/anpanel",
}

func safeFilePath(raw string) (string, error) {
	if raw == "" {
		return "/var/www", nil
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
		return "", errors.New("path is outside allowed roots (/var/www, /www, /srv, /opt, /home, /etc/anpanel/compose, /var/lib/anpanel)")
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

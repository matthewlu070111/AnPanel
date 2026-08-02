package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPasswordRoundTrip(t *testing.T) {
	h, err := HashPassword("correct-horse-battery")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword("correct-horse-battery", h) {
		t.Fatal("correct password rejected")
	}
	if VerifyPassword("wrong-password", h) {
		t.Fatal("wrong password accepted")
	}
}
func TestStoreAuthenticationAndSession(t *testing.T) {
	dir, err := os.MkdirTemp(".", ".anpanel-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	s, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	created, err := s.EnsureAdmin("admin", "initial-password")
	if err != nil || !created {
		t.Fatalf("create admin: %v", err)
	}
	u, err := s.Authenticate("admin", "initial-password")
	if err != nil {
		t.Fatal(err)
	}
	ss, err := s.CreateSession(u)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := s.Session(ss.Token)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Token != ss.Token || loaded.User.Username != "admin" {
		t.Fatalf("bad session: %#v", loaded)
	}
	if err := s.DeleteSession(ss.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Session(ss.Token); err == nil {
		t.Fatal("deleted session still valid")
	}
}

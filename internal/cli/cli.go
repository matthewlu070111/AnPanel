package cli

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/matthewlu070111/anpanel/internal/config"
	"github.com/matthewlu070111/anpanel/internal/store"
)

func Run(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: anpanel ctl <init-admin|show-port|reset-admin|disable-totp|recover-access>")
		return 2
	}
	cfg, err := config.Load()
	if err != nil {
		return fail(err)
	}
	switch args[0] {
	case "show-port":
		fmt.Println(cfg.Port)
	case "recover-access":
		cfg.Listen = "0.0.0.0"
		if err := config.Save(cfg); err != nil {
			return fail(err)
		}
		fmt.Printf("access restored at http://0.0.0.0:%d\n", cfg.Port)
	case "init-admin":
		db, err := store.Open(cfg.DatabasePath)
		if err != nil {
			return fail(err)
		}
		defer db.Close()
		password := randomPassword(12)
		created, err := db.EnsureAdmin("admin", password)
		if err != nil {
			return fail(err)
		}
		if !created {
			fmt.Println("administrator already exists; credentials unchanged")
			break
		}
		fmt.Printf("username: admin\npassword: %s\n", password)
	case "reset-admin":
		db, err := store.Open(cfg.DatabasePath)
		if err != nil {
			return fail(err)
		}
		defer db.Close()
		password := randomPassword(12)
		if err := db.ResetAdmin("admin", password); err != nil {
			return fail(err)
		}
		fmt.Printf("username: admin\npassword: %s\n", password)
	case "disable-totp":
		db, err := store.Open(cfg.DatabasePath)
		if err != nil {
			return fail(err)
		}
		defer db.Close()
		if err := db.DisableTOTP(); err != nil {
			return fail(err)
		}
		fmt.Println("TOTP disabled")
	default:
		return fail(fmt.Errorf("unknown command %q", args[0]))
	}
	return 0
}

func fail(err error) int { fmt.Fprintln(os.Stderr, err); return 1 }
func randomPassword(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)[:n]
}

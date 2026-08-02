package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/matthewlu070111/anpanel/internal/domain"
	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"golang.org/x/crypto/argon2"
)

type Store struct{ db *sql.DB }

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
PRAGMA journal_mode=WAL;
PRAGMA busy_timeout=5000;
PRAGMA foreign_keys=ON;
CREATE TABLE IF NOT EXISTS users (
 id INTEGER PRIMARY KEY CHECK(id=1), username TEXT NOT NULL UNIQUE,
 password_hash TEXT NOT NULL, must_change INTEGER NOT NULL DEFAULT 1,
 totp_secret TEXT NOT NULL DEFAULT '', created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS sessions (
 token_hash TEXT PRIMARY KEY, user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 csrf_token TEXT NOT NULL, expires_at INTEGER NOT NULL, created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS metrics (
 ts INTEGER PRIMARY KEY, cpu REAL NOT NULL, load1 REAL NOT NULL,
 memory_total INTEGER NOT NULL, memory_used INTEGER NOT NULL,
 swap_total INTEGER NOT NULL, swap_used INTEGER NOT NULL,
 disk_total INTEGER NOT NULL, disk_used INTEGER NOT NULL,
 net_rx INTEGER NOT NULL, net_tx INTEGER NOT NULL, uptime INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS metrics_5m (
 ts INTEGER PRIMARY KEY, cpu REAL NOT NULL, load1 REAL NOT NULL,
 memory_total INTEGER NOT NULL, memory_used INTEGER NOT NULL,
 swap_total INTEGER NOT NULL, swap_used INTEGER NOT NULL,
 disk_total INTEGER NOT NULL, disk_used INTEGER NOT NULL,
 net_rx INTEGER NOT NULL, net_tx INTEGER NOT NULL, uptime INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS tasks (
 id TEXT PRIMARY KEY, kind TEXT NOT NULL, status TEXT NOT NULL,
 summary TEXT NOT NULL, log TEXT NOT NULL DEFAULT '', result TEXT NOT NULL DEFAULT '{}',
 created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS audit_events (
 id INTEGER PRIMARY KEY AUTOINCREMENT, actor TEXT NOT NULL, action TEXT NOT NULL,
 resource TEXT NOT NULL, detail TEXT NOT NULL, remote_ip TEXT NOT NULL, created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS settings (key TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS alert_rules (
 id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL, metric TEXT NOT NULL,
 operator TEXT NOT NULL, threshold REAL NOT NULL, duration_seconds INTEGER NOT NULL,
 silence_seconds INTEGER NOT NULL, repeat_seconds INTEGER NOT NULL, enabled INTEGER NOT NULL DEFAULT 1
);`)
	return err
}

func (s *Store) EnsureAdmin(username, password string) (bool, error) {
	var n int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&n); err != nil {
		return false, err
	}
	if n > 0 {
		return false, nil
	}
	hash, err := HashPassword(password)
	if err != nil {
		return false, err
	}
	_, err = s.db.Exec("INSERT INTO users(id,username,password_hash,must_change,created_at) VALUES(1,?,?,1,?)", username, hash, time.Now().Unix())
	return err == nil, err
}

type User struct {
	ID         int64
	Username   string
	MustChange bool
	TOTPSecret string
}

func (s *Store) Authenticate(username, password string) (User, error) {
	var u User
	var hash string
	var must int
	err := s.db.QueryRow("SELECT id,username,password_hash,must_change,totp_secret FROM users WHERE username=?", username).
		Scan(&u.ID, &u.Username, &hash, &must, &u.TOTPSecret)
	if err != nil || !VerifyPassword(password, hash) {
		return User{}, errors.New("invalid credentials")
	}
	u.MustChange = must != 0
	return u, nil
}

func (s *Store) ChangeAdmin(username, password string) error {
	h, err := HashPassword(password)
	if err != nil {
		return err
	}
	_, err = s.db.Exec("UPDATE users SET username=?, password_hash=?, must_change=0 WHERE id=1", username, h)
	return err
}

func (s *Store) ResetAdmin(username, password string) error {
	h, err := HashPassword(password)
	if err != nil {
		return err
	}
	_, err = s.db.Exec("UPDATE users SET username=?,password_hash=?,must_change=1,totp_secret='' WHERE id=1", username, h)
	if err == nil {
		_, err = s.db.Exec("DELETE FROM sessions")
	}
	return err
}

func (s *Store) SetTOTP(secret string) error {
	_, err := s.db.Exec("UPDATE users SET totp_secret=? WHERE id=1", secret)
	return err
}
func (s *Store) DisableTOTP() error { return s.SetTOTP("") }

type Session struct {
	User        User
	CSRF, Token string
	ExpiresAt   time.Time
}

func (s *Store) CreateSession(user User) (Session, error) {
	token, csrf := randomToken(32), randomToken(24)
	expires := time.Now().Add(24 * time.Hour)
	_, err := s.db.Exec("INSERT INTO sessions(token_hash,user_id,csrf_token,expires_at,created_at) VALUES(?,?,?,?,?)", tokenHash(token), user.ID, csrf, expires.Unix(), time.Now().Unix())
	return Session{User: user, CSRF: csrf, Token: token, ExpiresAt: expires}, err
}

func (s *Store) Session(token string) (Session, error) {
	var ss Session
	ss.Token = token
	var must int
	var exp int64
	err := s.db.QueryRow(`SELECT u.id,u.username,u.must_change,u.totp_secret,s.csrf_token,s.expires_at
 FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=? AND s.expires_at>?`, tokenHash(token), time.Now().Unix()).
		Scan(&ss.User.ID, &ss.User.Username, &must, &ss.User.TOTPSecret, &ss.CSRF, &exp)
	ss.User.MustChange = must != 0
	ss.ExpiresAt = time.Unix(exp, 0)
	return ss, err
}

func (s *Store) DeleteSession(token string) error {
	_, err := s.db.Exec("DELETE FROM sessions WHERE token_hash=?", tokenHash(token))
	return err
}

func (s *Store) SaveMetric(m domain.HostSnapshot) error {
	// Real-time samples stay in memory; persistence is coalesced into one-minute buckets.
	ts := (m.Time.Unix() / 60) * 60
	_, err := s.db.Exec(`INSERT OR REPLACE INTO metrics VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`,
		ts, m.CPUPercent, m.Load1, m.MemoryTotal, m.MemoryUsed, m.SwapTotal, m.SwapUsed, m.DiskTotal, m.DiskUsed, m.NetRX, m.NetTX, m.Uptime)
	return err
}

func (s *Store) Metrics(since time.Time, limit int) ([]domain.HostSnapshot, error) {
	if limit < 1 || limit > 10000 {
		limit = 1440
	}
	cutoff := time.Now().Add(-7 * 24 * time.Hour).Unix()
	rows, err := s.db.Query(`SELECT ts,cpu,load1,memory_total,memory_used,swap_total,swap_used,disk_total,disk_used,net_rx,net_tx,uptime FROM (
 SELECT ts,cpu,load1,memory_total,memory_used,swap_total,swap_used,disk_total,disk_used,net_rx,net_tx,uptime FROM metrics WHERE ts>=?
 UNION ALL
 SELECT ts,cpu,load1,memory_total,memory_used,swap_total,swap_used,disk_total,disk_used,net_rx,net_tx,uptime FROM metrics_5m WHERE ts>=? AND ts<?
 ) ORDER BY ts DESC LIMIT ?`, since.Unix(), since.Unix(), cutoff, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.HostSnapshot{}
	for rows.Next() {
		var m domain.HostSnapshot
		var ts int64
		if err := rows.Scan(&ts, &m.CPUPercent, &m.Load1, &m.MemoryTotal, &m.MemoryUsed, &m.SwapTotal, &m.SwapUsed, &m.DiskTotal, &m.DiskUsed, &m.NetRX, &m.NetTX, &m.Uptime); err != nil {
			return nil, err
		}
		m.Time = time.Unix(ts, 0)
		items = append(items, m)
	}
	return items, rows.Err()
}

func (s *Store) PruneMetrics() error {
	sevenDays := time.Now().Add(-7 * 24 * time.Hour).Unix()
	ninetyDays := time.Now().Add(-90 * 24 * time.Hour).Unix()
	return s.WithTx(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT OR REPLACE INTO metrics_5m
 SELECT (ts/300)*300,AVG(cpu),AVG(load1),CAST(AVG(memory_total) AS INTEGER),CAST(AVG(memory_used) AS INTEGER),CAST(AVG(swap_total) AS INTEGER),CAST(AVG(swap_used) AS INTEGER),CAST(AVG(disk_total) AS INTEGER),CAST(AVG(disk_used) AS INTEGER),MAX(net_rx),MAX(net_tx),MAX(uptime)
 FROM metrics WHERE ts<? AND ts>=? GROUP BY (ts/300)*300`, sevenDays, ninetyDays)
		if err != nil {
			return err
		}
		if _, err = tx.Exec("DELETE FROM metrics WHERE ts < ?", sevenDays); err != nil {
			return err
		}
		_, err = tx.Exec("DELETE FROM metrics_5m WHERE ts < ?", ninetyDays)
		return err
	})
}

func (s *Store) CreateTask(t domain.Task) error {
	_, err := s.db.Exec("INSERT INTO tasks(id,kind,status,summary,created_at,updated_at) VALUES(?,?,?,?,?,?)", t.ID, t.Kind, t.Status, t.Summary, t.CreatedAt.Unix(), t.UpdatedAt.Unix())
	return err
}
func (s *Store) UpdateTask(id, status, log string, result any) error {
	b, _ := json.Marshal(result)
	_, err := s.db.Exec("UPDATE tasks SET status=?,log=?,result=?,updated_at=? WHERE id=?", status, log, string(b), time.Now().Unix(), id)
	return err
}
func (s *Store) Tasks(limit int) ([]domain.Task, error) {
	if limit < 1 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.Query("SELECT id,kind,status,summary,log,created_at,updated_at FROM tasks ORDER BY created_at DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Task{}
	for rows.Next() {
		var t domain.Task
		var a, b int64
		if err := rows.Scan(&t.ID, &t.Kind, &t.Status, &t.Summary, &t.Log, &a, &b); err != nil {
			return nil, err
		}
		t.CreatedAt = time.Unix(a, 0)
		t.UpdatedAt = time.Unix(b, 0)
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) Audit(actor, action, resource, detail, ip string) error {
	_, err := s.db.Exec("INSERT INTO audit_events(actor,action,resource,detail,remote_ip,created_at) VALUES(?,?,?,?,?,?)", actor, action, resource, detail, ip, time.Now().Unix())
	return err
}
func (s *Store) Audits(limit int) ([]domain.AuditEvent, error) {
	if limit < 1 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.Query("SELECT id,actor,action,resource,detail,remote_ip,created_at FROM audit_events ORDER BY id DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.AuditEvent{}
	for rows.Next() {
		var e domain.AuditEvent
		var ts int64
		if err := rows.Scan(&e.ID, &e.Actor, &e.Action, &e.Resource, &e.Detail, &e.RemoteIP, &ts); err != nil {
			return nil, err
		}
		e.CreatedAt = time.Unix(ts, 0)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) Setting(key string) (string, error) {
	var v string
	err := s.db.QueryRow("SELECT value FROM settings WHERE key=?", key).Scan(&v)
	return v, err
}
func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec("INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", key, value)
	return err
}

func (s *Store) AlertRules() ([]domain.AlertRule, error) {
	rows, err := s.db.Query("SELECT id,name,metric,operator,threshold,duration_seconds,silence_seconds,repeat_seconds,enabled FROM alert_rules ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.AlertRule{}
	for rows.Next() {
		var r domain.AlertRule
		var enabled int
		if err := rows.Scan(&r.ID, &r.Name, &r.Metric, &r.Operator, &r.Threshold, &r.DurationSeconds, &r.SilenceSeconds, &r.RepeatSeconds, &enabled); err != nil {
			return nil, err
		}
		r.Enabled = enabled != 0
		out = append(out, r)
	}
	return out, rows.Err()
}
func (s *Store) SaveAlertRule(r domain.AlertRule) (int64, error) {
	enabled := 0
	if r.Enabled {
		enabled = 1
	}
	if r.ID > 0 {
		_, err := s.db.Exec("UPDATE alert_rules SET name=?,metric=?,operator=?,threshold=?,duration_seconds=?,silence_seconds=?,repeat_seconds=?,enabled=? WHERE id=?", r.Name, r.Metric, r.Operator, r.Threshold, r.DurationSeconds, r.SilenceSeconds, r.RepeatSeconds, enabled, r.ID)
		return r.ID, err
	}
	res, err := s.db.Exec("INSERT INTO alert_rules(name,metric,operator,threshold,duration_seconds,silence_seconds,repeat_seconds,enabled) VALUES(?,?,?,?,?,?,?,?)", r.Name, r.Metric, r.Operator, r.Threshold, r.DurationSeconds, r.SilenceSeconds, r.RepeatSeconds, enabled)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}
func (s *Store) DeleteAlertRule(id int64) error {
	_, err := s.db.Exec("DELETE FROM alert_rules WHERE id=?", id)
	return err
}

func (s *Store) WithTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func HashPassword(password string) (string, error) {
	if len(password) < 10 {
		return "", errors.New("password must contain at least 10 characters")
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	h := argon2.IDKey([]byte(password), salt, 3, 64*1024, 2, 32)
	return fmt.Sprintf("$argon2id$v=19$m=65536,t=3,p=2$%s$%s", base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(h)), nil
}

func VerifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false
	}
	var mem, iterations uint64
	var parallel uint64
	for _, p := range strings.Split(parts[3], ",") {
		kv := strings.SplitN(p, "=", 2)
		if len(kv) != 2 {
			return false
		}
		v, err := strconv.ParseUint(kv[1], 10, 32)
		if err != nil {
			return false
		}
		switch kv[0] {
		case "m":
			mem = v
		case "t":
			iterations = v
		case "p":
			parallel = v
		}
	}
	if mem == 0 || iterations == 0 || parallel == 0 || parallel > 255 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, uint32(iterations), uint32(mem), uint8(parallel), uint32(len(expected)))
	return subtleEqual(actual, expected)
}

func randomToken(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
func tokenHash(v string) string {
	h := sha256.Sum256([]byte(v))
	return base64.RawURLEncoding.EncodeToString(h[:])
}
func subtleEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := range a {
		v |= a[i] ^ b[i]
	}
	return v == 0
}

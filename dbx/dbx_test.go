package dbx

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestConfigParseMySQL(t *testing.T) {
	cfg := (&Config{
		DSN:  "mysql://root:@127.0.0.1:3306/mysql",
		Pass: "123456",
		Args: map[string]string{
			"parseTime": "true",
			"loc":       "Local",
		},
	}).Parse()

	if cfg.scheme != "mysql" {
		t.Fatalf("expected scheme mysql, got %q", cfg.scheme)
	}

	got := cfg.String()
	wantParts := []string{
		"root:123456@tcp(127.0.0.1:3306)/mysql",
		"parseTime=true",
		"loc=Local",
	}
	for _, part := range wantParts {
		if !strings.Contains(got, part) {
			t.Fatalf("expected mysql dsn to contain %q, got %q", part, got)
		}
	}
}

func TestConfigParsePostgres(t *testing.T) {
	cfg := (&Config{
		DSN:  "postgres://user:oldpass@127.0.0.1:5432/postgres",
		User: "mozhu",
		Pass: "newpass",
		Name: "mulan",
		Args: map[string]string{
			"sslmode":  "disable",
			"TimeZone": "Asia/Shanghai",
		},
	}).Parse()

	if cfg.scheme != "postgres" {
		t.Fatalf("expected scheme postgres, got %q", cfg.scheme)
	}

	got := cfg.String()
	wantParts := []string{
		"postgres://mozhu:newpass@127.0.0.1:5432/mulan",
		"sslmode=disable",
		"TimeZone=Asia%2FShanghai",
	}
	for _, part := range wantParts {
		if !strings.Contains(got, part) {
			t.Fatalf("expected postgres dsn to contain %q, got %q", part, got)
		}
	}
}

func TestConfigParseSQLiteMemory(t *testing.T) {
	cfg := (&Config{
		DSN: "sqlite3://:memory:",
	}).Parse()

	if cfg.scheme != "sqlite3" {
		t.Fatalf("expected scheme sqlite3, got %q", cfg.scheme)
	}
	if cfg.String() != ":memory:" {
		t.Fatalf("expected sqlite memory dsn %q, got %q", ":memory:", cfg.String())
	}
}

func TestConfigParseSQLiteFile(t *testing.T) {
	cfg := (&Config{
		DSN: "sqlite3://testdata/app.db",
	}).Parse()

	if cfg.scheme != "sqlite3" {
		t.Fatalf("expected scheme sqlite3, got %q", cfg.scheme)
	}
	if cfg.String() != "testdata/app.db" {
		t.Fatalf("expected sqlite file dsn %q, got %q", "testdata/app.db", cfg.String())
	}
}

func TestConfigWithArgs(t *testing.T) {
	cfg := (&Config{}).
		WithArgs("sslmode", "disable").
		WithArgs("search_path", "public")

	if cfg.Args == nil {
		t.Fatal("expected args map to be initialized")
	}
	if cfg.Args["sslmode"] != "disable" {
		t.Fatalf("expected sslmode to be %q, got %q", "disable", cfg.Args["sslmode"])
	}
	if cfg.Args["search_path"] != "public" {
		t.Fatalf("expected search_path to be %q, got %q", "public", cfg.Args["search_path"])
	}
}

func TestAutoSQLiteMemory(t *testing.T) {
	db, err := Auto(&Config{
		DSN: "sqlite3://:memory:",
		Args: map[string]string{
			"cache": "shared",
		},
	})
	if err != nil {
		t.Fatalf("expected sqlite auto open success, got error: %v", err)
	}
	if db == nil {
		t.Fatal("expected db to be non-nil")
	}
}

func TestAutoUnsupportedScheme(t *testing.T) {
	_, err := Auto(&Config{
		DSN: "sqlserver://127.0.0.1/test",
	})
	if err == nil {
		t.Fatal("expected unsupported scheme error")
	}
	if !strings.Contains(err.Error(), "unsupported scheme") {
		t.Fatalf("expected unsupported scheme error, got %v", err)
	}
}

func TestModelBeforeCreateGeneratesUUID(t *testing.T) {
	var m Model

	if m.UUID != uuid.Nil {
		t.Fatalf("expected zero uuid before create, got %v", m.UUID)
	}

	if err := m.BeforeCreate(nil); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if m.UUID == uuid.Nil {
		t.Fatal("expected uuid to be generated")
	}
}

func TestModelDefaultsAndGetID(t *testing.T) {
	m := Model{ID: 42}

	if got := m.GetID(); got != 42 {
		t.Fatalf("expected id 42, got %d", got)
	}

	defaults := m.Defaults()
	if defaults == nil {
		t.Fatal("expected defaults slice, got nil")
	}
	if len(defaults) != 0 {
		t.Fatalf("expected empty defaults, got len=%d", len(defaults))
	}
}

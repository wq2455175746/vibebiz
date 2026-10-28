package migrations

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"

	"github.com/pressly/goose/v3"
)

var migrationName = regexp.MustCompile(`^\d{5}_[a-z0-9_]+\.sql$`)

func TestEmbeddedMigrationsHaveUpAndDown(t *testing.T) {
	entries, err := fs.ReadDir(FS, ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) < 5 {
		t.Fatalf("migration count = %d, want at least 5", len(entries))
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !migrationName.MatchString(entry.Name()) {
			t.Errorf("migration %q does not match %s", entry.Name(), migrationName)
		}
		body, err := fs.ReadFile(FS, entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		text := string(body)
		if !strings.Contains(text, "-- +goose Up") || !strings.Contains(text, "-- +goose Down") {
			t.Errorf("migration %q missing goose Up/Down directives", entry.Name())
		}
	}
}

func TestInitialSchemaCoversAllBusinessTables(t *testing.T) {
	tables := []string{
		"user", "ops_user", "ops_audit_log", "workspace", "workspace_user", "user_quota",
		"project_topic", "project_template", "project", "project_storage",
		"external_resource", "project_external_resource", "llm_model", "agent_role",
		"scheduled_task", "chat_conversation", "chat_message", "chat_memory",
		"rag_knowledge_base", "project_rag",
	}
	var all strings.Builder
	entries, err := fs.ReadDir(FS, ".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		body, err := fs.ReadFile(FS, entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		all.WriteString(strings.ToLower(string(body)))
	}
	schema := all.String()
	for _, table := range tables {
		if !strings.Contains(schema, "create table `"+table+"`") {
			t.Errorf("migration schema missing table %q", table)
		}
	}
}

func TestSeedMigrationContainsTopicsAndRoles(t *testing.T) {
	body, err := fs.ReadFile(FS, "00004_seed_topics_roles.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{"'common'", "'game'", "'video'", "'slides'", "'alex'", "'bob'", "'david'", "'emma'", "'mike'"} {
		if !strings.Contains(text, want) {
			t.Errorf("seed migration missing %s", want)
		}
	}
}

func TestDefaultUsersSeed(t *testing.T) {
	body, err := fs.ReadFile(FS, "00005_default_users.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{
		"wanghg11", "2455175746@qq.com",
		"vistor", "vistor_vibebiz@qq.com",
		"admin", "admin_vibebiz@qq.com",
		"super_admin", "personal", "owner",
		"`workspace`", "`workspace_user`", "`user_quota`", "`ops_user`",
		"NOT EXISTS",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("default user seed missing %q", want)
		}
	}
	if strings.Contains(text, "88888888") {
		t.Error("default user seed must not contain the plaintext password")
	}
	if matches := regexp.MustCompile(`\$2[aby]\$12\$`).FindAllString(text, -1); len(matches) != 3 {
		t.Errorf("bcrypt cost-12 hash count = %d, want 3", len(matches))
	}
}

func TestGooseCanCollectMigrations(t *testing.T) {
	goose.SetBaseFS(FS)
	migs, err := goose.CollectMigrations(".", 0, int64(^uint(0)>>1))
	if err != nil {
		t.Fatalf("collect migrations: %v", err)
	}
	if len(migs) != 5 {
		t.Fatalf("collected %d migrations, want 5", len(migs))
	}
	for i, mig := range migs {
		want := int64(i + 1)
		if mig.Version != want {
			t.Errorf("migration %d version = %d, want %d", i, mig.Version, want)
		}
	}
}

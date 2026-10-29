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
	if len(entries) < 6 {
		t.Fatalf("migration count = %d, want at least 6", len(entries))
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
		"sandbox_instance", "project_resource",
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

func TestSettingsMigrationAddsWorkspaceAvatarsAndQuota(t *testing.T) {
	body, err := fs.ReadFile(FS, "00006_settings.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{
		"ALTER TABLE `workspace`", "avatar_url", "avatar_s3_path",
		"storage_quota_bytes = 10737418240", "wanghg11", "vistor",
		"DROP COLUMN `avatar_s3_path`", "-- +goose Up", "-- +goose Down",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("settings migration missing %q", want)
		}
	}
}

func TestWorkspaceFolderMigration(t *testing.T) {
	body, err := fs.ReadFile(FS, "00007_workspace_folder.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{
		"CREATE TABLE `workspace_folder`", "workspace_id", "parent_id", "created_by",
		"uk_workspace_folder_name", "DROP TABLE IF EXISTS `workspace_folder`",
		"-- +goose Up", "-- +goose Down",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("folder migration missing %q", want)
		}
	}
}

func TestOpsNotificationMigration(t *testing.T) {
	body, err := fs.ReadFile(FS, "00008_ops_notification.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{
		"CREATE TABLE `ops_notification`", "title", "content", "expire_at", "published_at",
		"created_by_name", "idx_ops_notification_published", "DROP TABLE IF EXISTS `ops_notification`",
		"-- +goose Up", "-- +goose Down",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("notification migration missing %q", want)
		}
	}
}

func TestProjectPersistenceMigration(t *testing.T) {
	body, err := fs.ReadFile(FS, "00009_project_persistence.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{
		"ADD COLUMN `public_id` char(36)", "ADD COLUMN `prompt` mediumtext",
		"uk_project_public_id", "MODIFY COLUMN `public_id` char(36) NOT NULL",
		"MODIFY COLUMN `prompt` mediumtext NOT NULL",
		"-- +goose Up", "-- +goose Down",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("project persistence migration missing %q", want)
		}
	}
}

func TestSandboxRegistryMigration(t *testing.T) {
	body, err := fs.ReadFile(FS, "00010_sandbox_registry.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{
		"CREATE TABLE `sandbox_instance`", "active_requests", "max_concurrency",
		"last_heartbeat_at", "last_health_at", "ALTER TABLE `project`",
		"ADD COLUMN `sandbox_id`", "DROP TABLE IF EXISTS `sandbox_instance`",
		"-- +goose Up", "-- +goose Down",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("sandbox registry migration missing %q", want)
		}
	}
}

func TestGooseCanCollectMigrations(t *testing.T) {
	goose.SetBaseFS(FS)
	migs, err := goose.CollectMigrations(".", 0, int64(^uint(0)>>1))
	if err != nil {
		t.Fatalf("collect migrations: %v", err)
	}
	if len(migs) != 11 {
		t.Fatalf("collected %d migrations, want 11", len(migs))
	}
	for i, mig := range migs {
		want := int64(i + 1)
		if mig.Version != want {
			t.Errorf("migration %d version = %d, want %d", i, mig.Version, want)
		}
	}
}

func TestProjectResourceMigration(t *testing.T) {
	body, err := fs.ReadFile(FS, "00011_project_resource.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{
		"CREATE TABLE `project_resource`", "resource_id", "original_name",
		"display_name", "storage_name", "s3_staging_key", "s3_key", "status",
		"DROP TABLE IF EXISTS `project_resource`", "-- +goose Up", "-- +goose Down",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("project resource migration missing %q", want)
		}
	}
}

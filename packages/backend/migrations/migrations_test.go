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
		"rag_knowledge_base", "project_rag", "fde_resource",
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

func TestFdeResourceMigration(t *testing.T) {
	body, err := fs.ReadFile(FS, "00021_fde_resource.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{
		"CREATE TABLE `fde_resource`", "name", "contact", "skills", "region", "note",
		"enabled", "idx_fde_resource_enabled", "DROP TABLE IF EXISTS `fde_resource`",
		"-- +goose Up", "-- +goose Down",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("fde resource migration missing %q", want)
		}
	}
}

func TestGooseCanCollectMigrations(t *testing.T) {
	goose.SetBaseFS(FS)
	migs, err := goose.CollectMigrations(".", 0, int64(^uint(0)>>1))
	if err != nil {
		t.Fatalf("collect migrations: %v", err)
	}
	if len(migs) != 21 {
		t.Fatalf("collected %d migrations, want 21", len(migs))
	}
	for i, mig := range migs {
		want := int64(i + 1)
		if mig.Version != want {
			t.Errorf("migration %d version = %d, want %d", i, mig.Version, want)
		}
	}
}

func TestProjectAgentRoleMigration(t *testing.T) {
	body, err := fs.ReadFile(FS, "00012_project_agent_role.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{
		"CREATE TABLE `project_agent_role`",
		"`project_id` bigint UNSIGNED NOT NULL",
		"`agent_role_id` bigint UNSIGNED NOT NULL",
		"UNIQUE KEY `uk_project_agent_role` (`project_id`, `agent_role_id`)",
		"DROP TABLE IF EXISTS `project_agent_role`",
		"-- +goose Up",
		"-- +goose Down",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("project agent role migration missing %q", want)
		}
	}
}

func TestTeamRoleSeedMigration(t *testing.T) {
	body, err := fs.ReadFile(FS, "00013_seed_team_role.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{
		"'team'", "'Team'", "'团队'",
		"全部内置角色", "拆分", "子 Agent",
		"ON DUPLICATE KEY UPDATE", "-- +goose Up", "-- +goose Down",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("team seed migration missing %q", want)
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

func TestProjectCoverMigration(t *testing.T) {
	body, err := fs.ReadFile(FS, "00014_project_cover.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{
		"ALTER TABLE `project`",
		"ADD COLUMN `cover_s3_path`",
		"DROP COLUMN `cover_s3_path`",
		"-- +goose Up",
		"-- +goose Down",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("project cover migration missing %q", want)
		}
	}
}

func TestProjectModeMigration(t *testing.T) {
	body, err := fs.ReadFile(FS, "00016_project_mode.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{
		"ALTER TABLE `project`",
		"ADD COLUMN `mode` varchar(16) NOT NULL DEFAULT 'standard'",
		"DROP COLUMN `mode`",
		"-- +goose Up",
		"-- +goose Down",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("project mode migration missing %q", want)
		}
	}
}

func TestEnterpriseRoleSeedMigration(t *testing.T) {
	body, err := fs.ReadFile(FS, "00017_seed_enterprise_roles.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{"'senior'", "'高级工程师'", "'qa'", "'测试开发工程师'", "ON DUPLICATE KEY UPDATE", "-- +goose Up", "-- +goose Down"} {
		if !strings.Contains(text, want) {
			t.Errorf("enterprise role seed missing %q", want)
		}
	}
}

func TestProjectStageMigration(t *testing.T) {
	body, err := fs.ReadFile(FS, "00018_project_stage.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{
		"CREATE TABLE `project_stage`", "project_id", "stage_key", "artifact_path",
		"uk_project_stage", "DROP TABLE IF EXISTS `project_stage`",
		"-- +goose Up", "-- +goose Down",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("project stage migration missing %q", want)
		}
	}
}

func TestNotificationChannelMigration(t *testing.T) {
	body, err := fs.ReadFile(FS, "00019_notification_channel.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{
		"CREATE TABLE `notification_channel`", "user_id", "config_cipher",
		"uk_notification_channel_user_name", "last_test_status",
		"DROP TABLE IF EXISTS `notification_channel`", "-- +goose Up", "-- +goose Down",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("notification channel migration missing %q", want)
		}
	}
}

func TestScheduledTaskTopicMigration(t *testing.T) {
	body, err := fs.ReadFile(FS, "00020_scheduled_task_topic.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{"'scheduled'", "'定时任务'", "prompt_template", "-- +goose Up", "-- +goose Down"} {
		if !strings.Contains(text, want) {
			t.Errorf("scheduled task topic migration missing %q", want)
		}
	}
}

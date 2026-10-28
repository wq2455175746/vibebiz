package model

import "testing"

func TestTableNames(t *testing.T) {
	cases := []struct {
		model interface{ TableName() string }
		want  string
	}{
		{&User{}, "user"},
		{&OpsUser{}, "ops_user"},
		{&OpsAuditLog{}, "ops_audit_log"},
		{&Workspace{}, "workspace"},
		{&WorkspaceUser{}, "workspace_user"},
		{&UserQuota{}, "user_quota"},
		{&ProjectTopic{}, "project_topic"},
		{&ProjectTemplate{}, "project_template"},
		{&Project{}, "project"},
		{&ProjectStorage{}, "project_storage"},
		{&ExternalResource{}, "external_resource"},
		{&ProjectExternalResource{}, "project_external_resource"},
		{&LLMModel{}, "llm_model"},
		{&AgentRole{}, "agent_role"},
		{&ScheduledTask{}, "scheduled_task"},
		{&ChatConversation{}, "chat_conversation"},
		{&ChatMessage{}, "chat_message"},
		{&ChatMemory{}, "chat_memory"},
		{&RagKnowledgeBase{}, "rag_knowledge_base"},
		{&ProjectRag{}, "project_rag"},
	}
	for _, tc := range cases {
		if got := tc.model.TableName(); got != tc.want {
			t.Errorf("TableName() = %q, want %q", got, tc.want)
		}
	}
}

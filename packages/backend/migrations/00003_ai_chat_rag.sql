-- models, agent roles, scheduler, chat, memory and RAG

-- +goose Up

CREATE TABLE `llm_model` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `scope` varchar(16) NOT NULL DEFAULT 'user' COMMENT 'platform/user',
  `user_id` bigint UNSIGNED NOT NULL DEFAULT 0 COMMENT '用户ID；0=平台模型',
  `model_name` varchar(128) NOT NULL COMMENT '模型名称',
  `model_code` varchar(64) NOT NULL COMMENT '模型编码',
  `api_url` varchar(512) NOT NULL COMMENT 'API URL',
  `api_key_cipher` varchar(2048) NOT NULL COMMENT '加密后的 API Key',
  `provider_code` varchar(32) NOT NULL DEFAULT 'Other' COMMENT 'OpenAI/DeepSeek/Anthropic/Qwen/Other',
  `model_type` varchar(32) NOT NULL DEFAULT 'LLM' COMMENT 'LLM/EMBEDDING/RERANK/OCR/OTHER',
  `max_tokens` int NOT NULL DEFAULT 4096 COMMENT '最大输出 token',
  `context_window` int UNSIGNED DEFAULT NULL COMMENT '上下文窗口',
  `is_small_model` tinyint(1) NOT NULL DEFAULT 0 COMMENT '是否小模型',
  `is_default` tinyint(1) NOT NULL DEFAULT 0 COMMENT '是否默认模型',
  `established` tinyint(1) NOT NULL DEFAULT 0 COMMENT '是否连通：0断开，1连通',
  `status` tinyint UNSIGNED NOT NULL DEFAULT 1 COMMENT '状态：1启用，0禁用',
  `extra_params` json DEFAULT NULL COMMENT '模型扩展参数',
  `created_by` varchar(255) DEFAULT NULL COMMENT '创建者',
  `updated_by` varchar(255) DEFAULT NULL COMMENT '更新者',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` datetime DEFAULT NULL COMMENT 'gorm 软删除时间',
  `extra_p` json DEFAULT NULL COMMENT '扩展字段',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_llm_model_owner_code` (`scope`, `user_id`, `model_code`),
  KEY `idx_llm_model_owner_type` (`scope`, `user_id`, `model_type`, `status`),
  KEY `idx_llm_model_code` (`model_code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='LLM模型表';

CREATE TABLE `agent_role` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `role_code` varchar(32) NOT NULL COMMENT '角色编码：alex/bob/david/emma/mike',
  `role_name_en` varchar(64) NOT NULL COMMENT '英文名',
  `role_name_zh` varchar(64) NOT NULL COMMENT '中文名',
  `description` varchar(1000) DEFAULT NULL COMMENT '角色说明',
  `system_prompt` mediumtext COMMENT '角色提示词',
  `avatar_url` varchar(1024) DEFAULT NULL COMMENT '角色头像',
  `is_builtin` tinyint(1) NOT NULL DEFAULT 0 COMMENT '是否内置',
  `status` tinyint UNSIGNED NOT NULL DEFAULT 1 COMMENT '状态：1启用，0禁用',
  `sort_order` int NOT NULL DEFAULT 0 COMMENT '排序',
  `version` int UNSIGNED NOT NULL DEFAULT 1 COMMENT '提示词版本',
  `created_by` varchar(255) DEFAULT NULL COMMENT '创建者',
  `updated_by` varchar(255) DEFAULT NULL COMMENT '更新者',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` datetime DEFAULT NULL COMMENT 'gorm 软删除时间',
  `extra_p` json DEFAULT NULL COMMENT '扩展字段',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_agent_role_code` (`role_code`),
  KEY `idx_agent_role_status_sort` (`status`, `sort_order`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Agent角色表';

CREATE TABLE `scheduled_task` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `user_id` bigint UNSIGNED NOT NULL COMMENT '所属用户ID',
  `workspace_id` bigint UNSIGNED DEFAULT NULL COMMENT '可选工作区ID',
  `project_id` bigint UNSIGNED DEFAULT NULL COMMENT '可选项目ID',
  `name` varchar(128) NOT NULL COMMENT '任务名称',
  `task_type` varchar(32) NOT NULL DEFAULT 'agent' COMMENT 'agent/webhook/cleanup',
  `cron_expr` varchar(128) NOT NULL COMMENT 'cron 表达式',
  `timezone` varchar(64) NOT NULL DEFAULT 'Asia/Shanghai' COMMENT '时区',
  `prompt` mediumtext COMMENT 'Agent 任务提示词',
  `payload` json DEFAULT NULL COMMENT '任务参数：Webhook/清理配置等',
  `agent_role_id` bigint UNSIGNED DEFAULT NULL COMMENT 'Agent 角色ID',
  `model_id` bigint UNSIGNED DEFAULT NULL COMMENT '模型ID',
  `enabled` tinyint(1) NOT NULL DEFAULT 1 COMMENT '是否启用',
  `next_run_at` datetime DEFAULT NULL COMMENT '下次执行时间',
  `last_run_at` datetime DEFAULT NULL COMMENT '上次执行时间',
  `last_status` varchar(16) DEFAULT NULL COMMENT 'pending/running/success/failed',
  `last_error` varchar(1000) DEFAULT NULL COMMENT '最近错误',
  `run_count` int UNSIGNED NOT NULL DEFAULT 0 COMMENT '已执行次数',
  `max_runs` int UNSIGNED NOT NULL DEFAULT 0 COMMENT '最大次数，0 表示不限',
  `start_at` datetime DEFAULT NULL COMMENT '生效开始时间',
  `end_at` datetime DEFAULT NULL COMMENT '生效结束时间',
  `created_by` varchar(255) DEFAULT NULL COMMENT '创建者',
  `updated_by` varchar(255) DEFAULT NULL COMMENT '更新者',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` datetime DEFAULT NULL COMMENT 'gorm 软删除时间',
  `extra_p` json DEFAULT NULL COMMENT '扩展字段',
  PRIMARY KEY (`id`),
  KEY `idx_scheduled_task_due` (`enabled`, `next_run_at`),
  KEY `idx_scheduled_task_user` (`user_id`, `enabled`, `deleted_at`),
  KEY `idx_scheduled_task_project` (`project_id`, `deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户定时任务表';

CREATE TABLE `chat_conversation` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '会话ID',
  `workspace_id` bigint UNSIGNED NOT NULL COMMENT '工作区ID',
  `project_id` bigint UNSIGNED NOT NULL COMMENT '项目ID',
  `user_id` bigint UNSIGNED NOT NULL COMMENT '发起用户ID',
  `title` varchar(255) NOT NULL DEFAULT '新对话' COMMENT '会话标题',
  `model_id` bigint UNSIGNED NOT NULL COMMENT '模型ID',
  `agent_role_id` bigint UNSIGNED NOT NULL COMMENT 'Agent角色ID',
  `system_prompt` mediumtext COMMENT '自定义系统提示词',
  `source` varchar(32) NOT NULL DEFAULT 'client' COMMENT 'client/feishu/wechat/dingtalk',
  `ext_chat_id` varchar(255) DEFAULT NULL COMMENT '外部 IM 会话ID',
  `status` varchar(16) NOT NULL DEFAULT 'active' COMMENT 'active/archived',
  `message_count` int UNSIGNED NOT NULL DEFAULT 0 COMMENT '消息总数冗余',
  `total_tokens` bigint UNSIGNED NOT NULL DEFAULT 0 COMMENT '累计 Token',
  `last_message_at` datetime DEFAULT NULL COMMENT '最近消息时间',
  `start_at` datetime DEFAULT NULL COMMENT '会话开始时间',
  `end_at` datetime DEFAULT NULL COMMENT '会话结束时间',
  `parent_conversation_id` bigint UNSIGNED DEFAULT NULL COMMENT 'fork 来源会话ID',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` datetime DEFAULT NULL COMMENT 'gorm 软删除时间',
  `extra_p` json DEFAULT NULL COMMENT '扩展字段',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_chat_conversation_ext` (`source`, `ext_chat_id`),
  KEY `idx_chat_conversation_project` (`project_id`, `status`, `updated_at`),
  KEY `idx_chat_conversation_user` (`user_id`, `updated_at`),
  KEY `idx_chat_conversation_workspace` (`workspace_id`, `updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='项目级 AI 对话会话表';

CREATE TABLE `chat_message` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '消息ID',
  `conversation_id` bigint UNSIGNED NOT NULL COMMENT '所属会话ID',
  `role` varchar(16) NOT NULL COMMENT 'user/assistant/system/tool',
  `content` mediumtext NOT NULL COMMENT '消息内容',
  `attachments` json DEFAULT NULL COMMENT '附件列表',
  `commands` json DEFAULT NULL COMMENT '需要在客户端执行的 CLI 命令',
  `status` varchar(16) NOT NULL DEFAULT 'completed' COMMENT 'pending/streaming/completed/error/stopped',
  `error_message` text COMMENT '错误信息',
  `token_usage` int UNSIGNED DEFAULT NULL COMMENT '消耗 Token',
  `response_time_ms` int UNSIGNED DEFAULT NULL COMMENT '响应耗时毫秒',
  `parent_message_id` bigint UNSIGNED DEFAULT NULL COMMENT '父消息ID，用于分支/重新生成',
  `version` int UNSIGNED NOT NULL DEFAULT 1 COMMENT '消息版本',
  `model_id` bigint UNSIGNED DEFAULT NULL COMMENT '本条消息使用的模型ID',
  `commit_sha` varchar(64) DEFAULT NULL COMMENT '本轮产生的 git commit',
  `trace_id` varchar(64) DEFAULT NULL COMMENT 'Trace ID',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` datetime DEFAULT NULL COMMENT 'gorm 软删除时间',
  `extra_p` json DEFAULT NULL COMMENT '扩展字段',
  PRIMARY KEY (`id`),
  KEY `idx_chat_message_conversation` (`conversation_id`, `created_at`),
  KEY `idx_chat_message_parent` (`parent_message_id`),
  KEY `idx_chat_message_status` (`status`),
  KEY `idx_chat_message_trace` (`trace_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='对话消息表';

CREATE TABLE `chat_memory` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `scope` varchar(16) NOT NULL DEFAULT 'user' COMMENT 'user/project',
  `user_id` bigint UNSIGNED NOT NULL COMMENT '用户ID',
  `workspace_id` bigint UNSIGNED DEFAULT NULL COMMENT '工作区ID；项目级记忆必填',
  `project_id` bigint UNSIGNED DEFAULT NULL COMMENT '项目ID；项目级记忆必填',
  `name` varchar(255) NOT NULL COMMENT '记忆名称',
  `conversation_ids` json DEFAULT NULL COMMENT '关联会话ID列表',
  `mem` mediumtext NOT NULL COMMENT '记忆内容，Markdown',
  `mem_type` varchar(32) NOT NULL DEFAULT 'daily_summary' COMMENT 'daily_summary/topic_summary/key_fact/preference',
  `time_range_start_at` datetime DEFAULT NULL COMMENT '记忆覆盖开始时间',
  `time_range_end_at` datetime DEFAULT NULL COMMENT '记忆覆盖结束时间',
  `source_msg_count` int UNSIGNED NOT NULL DEFAULT 0 COMMENT '归纳涉及消息数',
  `version` int UNSIGNED NOT NULL DEFAULT 1 COMMENT '同类型记忆版本',
  `is_active` tinyint(1) NOT NULL DEFAULT 1 COMMENT '是否生效',
  `expire_at` datetime DEFAULT NULL COMMENT '过期时间，NULL 永久',
  `confidence_score` tinyint UNSIGNED DEFAULT NULL COMMENT '置信度 1-10',
  `created_by` varchar(255) DEFAULT NULL COMMENT '创建者：system/agent/user',
  `updated_by` varchar(255) DEFAULT NULL COMMENT '更新者',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` datetime DEFAULT NULL COMMENT 'gorm 软删除时间',
  `extra_p` json DEFAULT NULL COMMENT '扩展字段',
  PRIMARY KEY (`id`),
  KEY `idx_chat_memory_user_scope` (`user_id`, `scope`, `is_active`, `deleted_at`),
  KEY `idx_chat_memory_project_type` (`project_id`, `mem_type`, `is_active`, `deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户/项目记忆表';

CREATE TABLE `rag_knowledge_base` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '知识库ID',
  `user_id` bigint UNSIGNED NOT NULL COMMENT '用户ID',
  `name` varchar(255) NOT NULL COMMENT '知识库名',
  `rag_desc` varchar(1000) DEFAULT NULL COMMENT '知识库描述',
  `rag_metadata` json DEFAULT NULL COMMENT '元数据',
  `chunk_size` int UNSIGNED NOT NULL DEFAULT 500 COMMENT '文档分段大小',
  `overlap_size` int UNSIGNED NOT NULL DEFAULT 50 COMMENT '文档重叠大小',
  `dimension_size` int UNSIGNED NOT NULL DEFAULT 1024 COMMENT '向量维度',
  `embedding_model_id` bigint UNSIGNED NOT NULL COMMENT '嵌入模型ID',
  `embedding_model_name` varchar(255) NOT NULL COMMENT '嵌入模型名称快照',
  `reranker_model_id` bigint UNSIGNED DEFAULT NULL COMMENT 'Reranker 模型ID',
  `reranker_model_name` varchar(255) DEFAULT NULL COMMENT 'Reranker 模型名称快照',
  `doc_count` bigint UNSIGNED NOT NULL DEFAULT 0 COMMENT '文档数量',
  `collection_name` varchar(128) NOT NULL COMMENT '向量库集合名',
  `status` tinyint UNSIGNED NOT NULL DEFAULT 1 COMMENT '状态：1启用，0禁用',
  `created_by` varchar(255) DEFAULT NULL COMMENT '创建者',
  `updated_by` varchar(255) DEFAULT NULL COMMENT '更新者',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` datetime DEFAULT NULL COMMENT 'gorm 软删除时间',
  `extra_p` json DEFAULT NULL COMMENT '扩展字段',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_rag_collection` (`collection_name`),
  KEY `idx_rag_user_status` (`user_id`, `status`, `deleted_at`),
  KEY `idx_rag_embedding_model` (`embedding_model_id`),
  KEY `idx_rag_reranker_model` (`reranker_model_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='知识库管理表';

CREATE TABLE `project_rag` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `workspace_id` bigint UNSIGNED NOT NULL COMMENT '工作区ID',
  `project_id` bigint UNSIGNED NOT NULL COMMENT '项目ID',
  `rag_id` bigint UNSIGNED NOT NULL COMMENT '知识库ID',
  `created_by` varchar(255) DEFAULT NULL COMMENT '创建者',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` datetime DEFAULT NULL COMMENT 'gorm 软删除时间',
  `extra_p` json DEFAULT NULL COMMENT '扩展字段',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_project_rag` (`project_id`, `rag_id`),
  KEY `idx_project_rag_rag` (`rag_id`, `deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='项目知识库绑定表';

-- +goose Down
DROP TABLE IF EXISTS `project_rag`;
DROP TABLE IF EXISTS `rag_knowledge_base`;
DROP TABLE IF EXISTS `chat_memory`;
DROP TABLE IF EXISTS `chat_message`;
DROP TABLE IF EXISTS `chat_conversation`;
DROP TABLE IF EXISTS `scheduled_task`;
DROP TABLE IF EXISTS `agent_role`;
DROP TABLE IF EXISTS `llm_model`;

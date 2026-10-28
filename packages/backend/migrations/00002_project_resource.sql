-- project catalog, project storage and external resources

-- +goose Up

CREATE TABLE `project_topic` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `topic_code` varchar(32) NOT NULL COMMENT '主题编码：common/game/video/slides',
  `topic_name` varchar(64) NOT NULL COMMENT '主题名称',
  `description` varchar(1000) DEFAULT NULL COMMENT '主题说明',
  `prompt_template` mediumtext COMMENT '主题提示词模板',
  `icon` varchar(255) DEFAULT NULL COMMENT '图标 URL 或标识',
  `default_template_id` bigint UNSIGNED DEFAULT NULL COMMENT '默认模板ID',
  `is_default` tinyint(1) NOT NULL DEFAULT 0 COMMENT '是否默认主题',
  `sort_order` int NOT NULL DEFAULT 0 COMMENT '排序',
  `status` tinyint UNSIGNED NOT NULL DEFAULT 1 COMMENT '状态：1启用，0禁用',
  `created_by` varchar(255) DEFAULT NULL COMMENT '创建者',
  `updated_by` varchar(255) DEFAULT NULL COMMENT '更新者',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` datetime DEFAULT NULL COMMENT 'gorm 软删除时间',
  `extra_p` json DEFAULT NULL COMMENT '扩展字段',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_project_topic_code` (`topic_code`),
  KEY `idx_project_topic_status_sort` (`status`, `sort_order`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='工程主题表';

CREATE TABLE `project_template` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `scope` varchar(16) NOT NULL DEFAULT 'platform' COMMENT 'platform/workspace',
  `workspace_id` bigint UNSIGNED NOT NULL DEFAULT 0 COMMENT '工作区ID；0=平台模板',
  `template_code` varchar(64) NOT NULL COMMENT '模板编码',
  `template_name` varchar(128) NOT NULL COMMENT '模板名称',
  `description` varchar(1000) DEFAULT NULL COMMENT '模板描述',
  `topic_id` bigint UNSIGNED NOT NULL COMMENT '主题ID',
  `template_s3_path` varchar(1024) DEFAULT NULL COMMENT '模板包 S3 路径',
  `cover_url` varchar(1024) DEFAULT NULL COMMENT '封面 URL',
  `template_label` json DEFAULT NULL COMMENT '模板标签数组',
  `file_tree` json DEFAULT NULL COMMENT '模板文件树',
  `tech_stack` json DEFAULT NULL COMMENT '技术栈',
  `version` int UNSIGNED NOT NULL DEFAULT 1 COMMENT '模板版本',
  `is_builtin` tinyint(1) NOT NULL DEFAULT 0 COMMENT '是否内置',
  `status` tinyint UNSIGNED NOT NULL DEFAULT 1 COMMENT '状态：1启用，0禁用',
  `sort_order` int NOT NULL DEFAULT 0 COMMENT '排序',
  `created_by` varchar(255) DEFAULT NULL COMMENT '创建者',
  `updated_by` varchar(255) DEFAULT NULL COMMENT '更新者',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` datetime DEFAULT NULL COMMENT 'gorm 软删除时间',
  `extra_p` json DEFAULT NULL COMMENT '扩展字段',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_template_scope_owner_code` (`scope`, `workspace_id`, `template_code`),
  KEY `idx_template_topic_status` (`topic_id`, `status`, `sort_order`),
  KEY `idx_template_owner_status` (`scope`, `workspace_id`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='工程模板表';

CREATE TABLE `project` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `workspace_id` bigint UNSIGNED NOT NULL COMMENT '所属工作区ID',
  `user_id` bigint UNSIGNED NOT NULL COMMENT '创建者/负责人用户ID',
  `name` varchar(255) NOT NULL COMMENT '项目名',
  `description` varchar(2000) DEFAULT NULL COMMENT '项目描述',
  `topic_id` bigint UNSIGNED NOT NULL COMMENT '主题ID，默认 common',
  `template_id` bigint UNSIGNED DEFAULT NULL COMMENT '创建时使用的模板ID',
  `s3_path` varchar(1024) DEFAULT NULL COMMENT '项目 S3 前缀',
  `local_path` varchar(1024) DEFAULT NULL COMMENT '项目本地/RWX 路径',
  `project_structure` json DEFAULT NULL COMMENT '项目结构快照：页面/路由/文件树',
  `is_public` tinyint(1) NOT NULL DEFAULT 0 COMMENT '是否公开：0私有，1公开',
  `status` varchar(16) NOT NULL DEFAULT 'active' COMMENT 'draft/active/archived',
  `default_agent_role_id` bigint UNSIGNED DEFAULT NULL COMMENT '默认 Agent 角色ID',
  `last_active_at` datetime DEFAULT NULL COMMENT '最近活跃时间',
  `archived_at` datetime DEFAULT NULL COMMENT '归档时间',
  `created_by` varchar(255) DEFAULT NULL COMMENT '创建者',
  `updated_by` varchar(255) DEFAULT NULL COMMENT '更新者',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` datetime DEFAULT NULL COMMENT 'gorm 软删除时间',
  `extra_p` json DEFAULT NULL COMMENT '扩展字段',
  PRIMARY KEY (`id`),
  KEY `idx_project_workspace_updated` (`workspace_id`, `deleted_at`, `updated_at`),
  KEY `idx_project_user` (`user_id`, `deleted_at`),
  KEY `idx_project_topic` (`topic_id`),
  KEY `idx_project_public` (`is_public`, `updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='项目表';

CREATE TABLE `project_storage` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `workspace_id` bigint UNSIGNED NOT NULL COMMENT '工作区ID',
  `project_id` bigint UNSIGNED NOT NULL COMMENT '项目ID',
  `user_id` bigint UNSIGNED NOT NULL COMMENT '归属用户ID/创建者',
  `storage_path` varchar(1024) DEFAULT NULL COMMENT '文件存储地址：S3 前缀或本地路径',
  `file_size_bytes` bigint UNSIGNED NOT NULL DEFAULT 0 COMMENT '文件总大小',
  `file_count` int UNSIGNED NOT NULL DEFAULT 0 COMMENT '文件数',
  `dir_count` int UNSIGNED NOT NULL DEFAULT 0 COMMENT '目录数',
  `file_tree` json DEFAULT NULL COMMENT '文件树快照',
  `last_modified_at` datetime DEFAULT NULL COMMENT '文件最后修改时间',
  `last_scanned_at` datetime DEFAULT NULL COMMENT '最近扫描时间',
  `status` tinyint UNSIGNED NOT NULL DEFAULT 1 COMMENT '状态：1正常，0异常',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` datetime DEFAULT NULL COMMENT 'gorm 软删除时间',
  `extra_p` json DEFAULT NULL COMMENT '扩展字段',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_project_storage_project` (`project_id`),
  KEY `idx_project_storage_workspace` (`workspace_id`, `deleted_at`),
  KEY `idx_project_storage_user` (`user_id`, `deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='项目存储用量表';

CREATE TABLE `external_resource` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `workspace_id` bigint UNSIGNED NOT NULL COMMENT '所属工作区ID',
  `user_id` bigint UNSIGNED NOT NULL COMMENT '创建者用户ID',
  `name` varchar(128) NOT NULL COMMENT '资源名称',
  `resource_type` varchar(32) NOT NULL COMMENT 's3/github/obsidian/dropbox/url',
  `resource_url` varchar(2048) NOT NULL COMMENT '资源 URL/仓库地址',
  `auth_type` varchar(32) NOT NULL DEFAULT 'none' COMMENT 'none/token/oauth/basic',
  `credential_cipher` text COMMENT '加密后的凭据，禁止明文',
  `extra_configs` json DEFAULT NULL COMMENT '非敏感扩展配置',
  `sync_status` varchar(16) NOT NULL DEFAULT 'pending' COMMENT 'pending/syncing/success/failed',
  `last_sync_at` datetime DEFAULT NULL COMMENT '最近同步时间',
  `last_error` varchar(1000) DEFAULT NULL COMMENT '最近同步错误',
  `status` tinyint UNSIGNED NOT NULL DEFAULT 1 COMMENT '状态：1启用，0禁用',
  `created_by` varchar(255) DEFAULT NULL COMMENT '创建者',
  `updated_by` varchar(255) DEFAULT NULL COMMENT '更新者',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` datetime DEFAULT NULL COMMENT 'gorm 软删除时间',
  `extra_p` json DEFAULT NULL COMMENT '扩展字段',
  PRIMARY KEY (`id`),
  KEY `idx_external_workspace_type` (`workspace_id`, `resource_type`, `status`, `deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='外接资源表';

CREATE TABLE `project_external_resource` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `workspace_id` bigint UNSIGNED NOT NULL COMMENT '工作区ID',
  `project_id` bigint UNSIGNED NOT NULL COMMENT '项目ID',
  `resource_id` bigint UNSIGNED NOT NULL COMMENT '外接资源ID',
  `created_by` varchar(255) DEFAULT NULL COMMENT '创建者',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` datetime DEFAULT NULL COMMENT 'gorm 软删除时间',
  `extra_p` json DEFAULT NULL COMMENT '扩展字段',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_project_external_resource` (`project_id`, `resource_id`),
  KEY `idx_project_external_resource_resource` (`resource_id`, `deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='项目外接资源绑定表';

-- +goose Down
DROP TABLE IF EXISTS `project_external_resource`;
DROP TABLE IF EXISTS `external_resource`;
DROP TABLE IF EXISTS `project_storage`;
DROP TABLE IF EXISTS `project`;
DROP TABLE IF EXISTS `project_template`;
DROP TABLE IF EXISTS `project_topic`;

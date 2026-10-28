-- identity, ops identity, workspace and quota

-- +goose Up

CREATE TABLE `user` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `email` varchar(255) NOT NULL COMMENT '邮箱',
  `source` varchar(32) NOT NULL DEFAULT 'CUSTOM' COMMENT '来源：CUSTOM/OAUTH',
  `username` varchar(255) NOT NULL COMMENT '用户名',
  `password_hash` varchar(255) DEFAULT NULL COMMENT '密码哈希；OAuth 用户可为空',
  `nickname` varchar(255) DEFAULT NULL COMMENT '昵称',
  `phone_number` varchar(32) DEFAULT NULL COMMENT '手机号',
  `avatar_url` varchar(1024) DEFAULT NULL COMMENT '头像 URL',
  `avatar_s3_path` varchar(1024) DEFAULT NULL COMMENT '头像 S3 对象路径',
  `status` tinyint UNSIGNED NOT NULL DEFAULT 1 COMMENT '状态：1正常，0禁用',
  `email_verified_at` datetime DEFAULT NULL COMMENT '邮箱验证时间',
  `last_login_time` datetime DEFAULT NULL COMMENT '最后登录时间',
  `last_login_ip` varchar(64) DEFAULT NULL COMMENT '最后登录 IP',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` datetime DEFAULT NULL COMMENT 'gorm 软删除时间',
  `extra_p` json DEFAULT NULL COMMENT '扩展字段，禁止存放密钥',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_email` (`email`),
  UNIQUE KEY `uk_user_username` (`username`),
  KEY `idx_user_status_created` (`status`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='业务用户表';

CREATE TABLE `ops_user` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `username` varchar(255) NOT NULL COMMENT '运营用户名',
  `email` varchar(255) NOT NULL COMMENT '运营邮箱',
  `password_hash` varchar(255) DEFAULT NULL COMMENT '密码哈希；SSO 可为空',
  `nickname` varchar(255) DEFAULT NULL COMMENT '昵称',
  `phone_number` varchar(32) DEFAULT NULL COMMENT '手机号',
  `avatar_url` varchar(1024) DEFAULT NULL COMMENT '头像 URL',
  `role_code` varchar(32) NOT NULL DEFAULT 'operator' COMMENT '角色：super_admin/operator/viewer',
  `source` varchar(32) NOT NULL DEFAULT 'local' COMMENT '来源：local/SSO',
  `status` tinyint UNSIGNED NOT NULL DEFAULT 1 COMMENT '状态：1正常，0禁用',
  `last_login_time` datetime DEFAULT NULL COMMENT '最后登录时间',
  `last_login_ip` varchar(64) DEFAULT NULL COMMENT '最后登录 IP',
  `created_by` varchar(255) DEFAULT NULL COMMENT '创建者',
  `updated_by` varchar(255) DEFAULT NULL COMMENT '更新者',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` datetime DEFAULT NULL COMMENT 'gorm 软删除时间',
  `extra_p` json DEFAULT NULL COMMENT '扩展字段',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_ops_user_email` (`email`),
  UNIQUE KEY `uk_ops_user_username` (`username`),
  KEY `idx_ops_user_role_status` (`role_code`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='运营后台用户表';

CREATE TABLE `ops_audit_log` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `ops_user_id` bigint UNSIGNED DEFAULT NULL COMMENT '运营用户ID；系统操作可为空',
  `actor_name` varchar(255) DEFAULT NULL COMMENT '操作者名称快照',
  `action` varchar(128) NOT NULL COMMENT '操作：user.disable/project.delete/model.update',
  `resource_type` varchar(64) NOT NULL COMMENT '资源类型',
  `resource_id` varchar(64) DEFAULT NULL COMMENT '资源ID',
  `request_method` varchar(16) DEFAULT NULL COMMENT 'HTTP 方法',
  `request_path` varchar(512) DEFAULT NULL COMMENT '请求路径',
  `request_id` varchar(64) DEFAULT NULL COMMENT '请求ID',
  `trace_id` varchar(64) DEFAULT NULL COMMENT 'Trace ID',
  `ip` varchar(64) DEFAULT NULL COMMENT '来源 IP',
  `user_agent` varchar(512) DEFAULT NULL COMMENT 'User-Agent',
  `before_data` json DEFAULT NULL COMMENT '变更前快照，敏感字段脱敏',
  `after_data` json DEFAULT NULL COMMENT '变更后快照，敏感字段脱敏',
  `result_status` varchar(16) NOT NULL DEFAULT 'success' COMMENT 'success/failed',
  `error_message` varchar(1000) DEFAULT NULL COMMENT '错误信息',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` datetime DEFAULT NULL COMMENT '逻辑删除时间',
  `extra_p` json DEFAULT NULL COMMENT '扩展字段',
  PRIMARY KEY (`id`),
  KEY `idx_ops_audit_user_time` (`ops_user_id`, `created_at`),
  KEY `idx_ops_audit_resource` (`resource_type`, `resource_id`),
  KEY `idx_ops_audit_request` (`request_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='运营操作审计表';

CREATE TABLE `workspace` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `owner_user_id` bigint UNSIGNED NOT NULL COMMENT '工作区所有者用户ID',
  `name` varchar(128) NOT NULL COMMENT '工作区名称',
  `description` varchar(1000) DEFAULT NULL COMMENT '工作区描述',
  `workspace_type` varchar(16) NOT NULL DEFAULT 'personal' COMMENT 'personal/team',
  `visibility` varchar(16) NOT NULL DEFAULT 'private' COMMENT 'private/shared',
  `s3_prefix` varchar(1024) DEFAULT NULL COMMENT '工作区 S3 前缀',
  `local_path` varchar(1024) DEFAULT NULL COMMENT '工作区本地/RWX 路径',
  `status` tinyint UNSIGNED NOT NULL DEFAULT 1 COMMENT '状态：1正常，0禁用',
  `created_by` varchar(255) DEFAULT NULL COMMENT '创建者',
  `updated_by` varchar(255) DEFAULT NULL COMMENT '更新者',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` datetime DEFAULT NULL COMMENT 'gorm 软删除时间',
  `extra_p` json DEFAULT NULL COMMENT '扩展字段',
  PRIMARY KEY (`id`),
  KEY `idx_workspace_owner` (`owner_user_id`, `deleted_at`),
  KEY `idx_workspace_type_status` (`workspace_type`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='工作区表';

CREATE TABLE `workspace_user` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `workspace_id` bigint UNSIGNED NOT NULL COMMENT '工作区ID',
  `user_id` bigint UNSIGNED NOT NULL COMMENT '用户ID',
  `role` varchar(16) NOT NULL DEFAULT 'editor' COMMENT 'owner/admin/editor/viewer',
  `status` tinyint UNSIGNED NOT NULL DEFAULT 1 COMMENT '状态：1正常，0禁用',
  `invited_by` bigint UNSIGNED DEFAULT NULL COMMENT '邀请人用户ID',
  `joined_at` datetime DEFAULT NULL COMMENT '加入时间',
  `last_access_at` datetime DEFAULT NULL COMMENT '最近访问时间',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` datetime DEFAULT NULL COMMENT 'gorm 软删除时间',
  `extra_p` json DEFAULT NULL COMMENT '扩展字段',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_workspace_user` (`workspace_id`, `user_id`),
  KEY `idx_workspace_user_user` (`user_id`, `status`, `deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='工作区用户绑定表';

CREATE TABLE `user_quota` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `user_id` bigint UNSIGNED NOT NULL COMMENT '用户ID',
  `plan_code` varchar(32) NOT NULL DEFAULT 'free' COMMENT '套餐：free/pro/max',
  `storage_quota_bytes` bigint UNSIGNED NOT NULL DEFAULT 0 COMMENT '存储额度，0 表示不限',
  `storage_used_bytes` bigint UNSIGNED NOT NULL DEFAULT 0 COMMENT '已用存储字节',
  `project_quota` int UNSIGNED NOT NULL DEFAULT 0 COMMENT '项目额度，0 表示不限',
  `project_used` int UNSIGNED NOT NULL DEFAULT 0 COMMENT '已用项目数',
  `workspace_quota` int UNSIGNED NOT NULL DEFAULT 0 COMMENT '工作区额度，0 表示不限',
  `workspace_used` int UNSIGNED NOT NULL DEFAULT 0 COMMENT '已用工作区数',
  `llm_token_quota_month` bigint UNSIGNED NOT NULL DEFAULT 0 COMMENT '月度 Token 额度，0 表示不限',
  `llm_tokens_used_month` bigint UNSIGNED NOT NULL DEFAULT 0 COMMENT '本月已用 Token',
  `sandbox_minutes_quota_month` int UNSIGNED NOT NULL DEFAULT 0 COMMENT '月度沙箱分钟额度，0 表示不限',
  `sandbox_minutes_used_month` int UNSIGNED NOT NULL DEFAULT 0 COMMENT '本月已用沙箱分钟',
  `concurrent_agent_runs` int UNSIGNED NOT NULL DEFAULT 1 COMMENT '并发 Agent 运行上限',
  `reset_at` datetime DEFAULT NULL COMMENT '月度额度重置时间',
  `status` tinyint UNSIGNED NOT NULL DEFAULT 1 COMMENT '状态：1正常，0冻结',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` datetime DEFAULT NULL COMMENT 'gorm 软删除时间',
  `extra_p` json DEFAULT NULL COMMENT '扩展字段',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_quota_user` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户额度与聚合用量表';

-- +goose Down
DROP TABLE IF EXISTS `user_quota`;
DROP TABLE IF EXISTS `workspace_user`;
DROP TABLE IF EXISTS `workspace`;
DROP TABLE IF EXISTS `ops_audit_log`;
DROP TABLE IF EXISTS `ops_user`;
DROP TABLE IF EXISTS `user`;

-- +goose Up

CREATE TABLE `evolution_version` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `public_id` varchar(64) NOT NULL COMMENT '版本公开ID',
  `user_id` varchar(64) NOT NULL COMMENT '用户ID（official 表示官方基线）',
  `user_name` varchar(128) NOT NULL COMMENT '用户名称',
  `label` varchar(64) NOT NULL COMMENT '版本标签',
  `status` varchar(16) NOT NULL DEFAULT 'draft' COMMENT 'draft/preview/accepted/submitted/released',
  `scenario` varchar(64) NOT NULL DEFAULT 'baseline' COMMENT '预览场景',
  `summary` varchar(512) NOT NULL DEFAULT '' COMMENT '变更摘要',
  `origin_version` varchar(64) NOT NULL DEFAULT '' COMMENT '官方基线版本',
  `proposal_ids` json DEFAULT NULL COMMENT '关联提议ID',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` datetime DEFAULT NULL COMMENT 'gorm 软删除时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_evolution_version_public` (`public_id`),
  KEY `idx_evolution_version_user` (`user_id`, `updated_at`),
  KEY `idx_evolution_version_status` (`status`, `updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='集思进化用户版本';

CREATE TABLE `evolution_proposal` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `public_id` varchar(64) NOT NULL COMMENT '提议公开ID',
  `user_id` varchar(64) NOT NULL COMMENT '用户ID',
  `user_name` varchar(128) NOT NULL COMMENT '用户名称',
  `kind` varchar(16) NOT NULL COMMENT 'bug/visual/feature',
  `content` text NOT NULL COMMENT '提议内容',
  `status` varchar(16) NOT NULL DEFAULT 'queued' COMMENT 'queued/reviewing/accepted/rejected/released',
  `agent_reply` text NOT NULL COMMENT 'Agent 评估回复',
  `version_public_id` varchar(64) NOT NULL DEFAULT '' COMMENT '关联版本公开ID',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` datetime DEFAULT NULL COMMENT 'gorm 软删除时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_evolution_proposal_public` (`public_id`),
  KEY `idx_evolution_proposal_user` (`user_id`, `created_at`),
  KEY `idx_evolution_proposal_version` (`version_public_id`),
  KEY `idx_evolution_proposal_status` (`status`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='集思进化提议队列';

CREATE TABLE `evolution_message` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `public_id` varchar(64) NOT NULL COMMENT '消息公开ID',
  `user_id` varchar(64) NOT NULL COMMENT '所属用户ID',
  `user_name` varchar(128) NOT NULL COMMENT '展示名称',
  `role` varchar(16) NOT NULL COMMENT 'user/agent/admin',
  `content` text NOT NULL COMMENT '消息内容',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `deleted_at` datetime DEFAULT NULL COMMENT 'gorm 软删除时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_evolution_message_public` (`public_id`),
  KEY `idx_evolution_message_user` (`user_id`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='集思进化聊天记录';

CREATE TABLE `evolution_task` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `public_id` varchar(64) NOT NULL COMMENT '任务公开ID',
  `version_public_id` varchar(64) NOT NULL COMMENT '版本公开ID',
  `user_id` varchar(64) NOT NULL COMMENT '用户ID',
  `user_name` varchar(128) NOT NULL COMMENT '用户名称',
  `version_label` varchar(64) NOT NULL DEFAULT '' COMMENT '版本标签',
  `origin_version` varchar(64) NOT NULL DEFAULT '' COMMENT '官方基线版本',
  `chat_summary` text NOT NULL COMMENT '对话摘要',
  `status` varchar(16) NOT NULL DEFAULT 'queued' COMMENT 'queued/coding/ci/released/failed',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` datetime DEFAULT NULL COMMENT 'gorm 软删除时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_evolution_task_public` (`public_id`),
  KEY `idx_evolution_task_version` (`version_public_id`),
  KEY `idx_evolution_task_status` (`status`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='集思进化自动编码任务';

INSERT INTO `evolution_version`
  (`public_id`, `user_id`, `user_name`, `label`, `status`, `scenario`, `summary`, `origin_version`, `proposal_ids`, `created_at`, `updated_at`)
VALUES
  ('v-official-1', 'official', 'Vibebiz 官方', 'v1.2026.11', 'released', 'baseline', '当前线上版本的交互副本，由 CI 构建后写入 origin_version。', 'v1.2026.11', JSON_ARRAY(), DATE_SUB(NOW(), INTERVAL 8 DAY), DATE_SUB(NOW(), INTERVAL 8 DAY));

-- +goose Down

DROP TABLE IF EXISTS `evolution_task`;
DROP TABLE IF EXISTS `evolution_message`;
DROP TABLE IF EXISTS `evolution_proposal`;
DROP TABLE IF EXISTS `evolution_version`;

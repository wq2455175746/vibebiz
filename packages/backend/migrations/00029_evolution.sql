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
  ('v-official-1', 'official', 'Vibebiz 官方', 'v1.2026.11', 'released', 'baseline', '当前线上版本的交互副本，由 CI 构建后写入 origin_version。', 'v1.2026.11', JSON_ARRAY(), DATE_SUB(NOW(), INTERVAL 8 DAY), DATE_SUB(NOW(), INTERVAL 8 DAY)),
  ('v-1001-2', '1001', '林岚', 'v1001.2', 'preview', 'compact-sidebar', '收紧侧栏与项目卡片密度，修复删除按钮无响应。', 'v1.2026.11', JSON_ARRAY('p-1001-1', 'p-1001-2'), DATE_SUB(NOW(), INTERVAL 2 HOUR), DATE_SUB(NOW(), INTERVAL 2 HOUR)),
  ('v-1001-1', '1001', '林岚', 'v1001.1', 'accepted', 'soft-cards', '项目卡片改为低对比度圆角样式。', 'v1.2026.11', JSON_ARRAY(), DATE_SUB(NOW(), INTERVAL 1 DAY), DATE_SUB(NOW(), INTERVAL 1 DAY)),
  ('v-1002-1', '1002', '陈墨', 'v1002.1', 'preview', 'focus-composer', '首页首屏聚焦创作输入，弱化发现区视觉噪音。', 'v1.2026.11', JSON_ARRAY('p-1002-1'), DATE_SUB(NOW(), INTERVAL 5 HOUR), DATE_SUB(NOW(), INTERVAL 5 HOUR)),
  ('v-1003-1', '1003', '周野', 'v1003.1', 'submitted', 'quick-actions', '创作输入下增加快捷操作入口。', 'v1.2026.11', JSON_ARRAY('p-1003-1'), DATE_SUB(NOW(), INTERVAL 26 HOUR), DATE_SUB(NOW(), INTERVAL 26 HOUR)),
  ('v-1004-1', '1004', '苏晴', 'v1004.1', 'preview', 'soft-cards', '调整项目卡片层级与模板区留白。', 'v1.2026.11', JSON_ARRAY(), DATE_SUB(NOW(), INTERVAL 3 HOUR), DATE_SUB(NOW(), INTERVAL 3 HOUR));

INSERT INTO `evolution_proposal`
  (`public_id`, `user_id`, `user_name`, `kind`, `content`, `status`, `agent_reply`, `version_public_id`, `created_at`, `updated_at`)
VALUES
  ('p-1001-1', '1001', '林岚', 'bug', '首页项目卡片的删除按钮点击没有反应。', 'accepted', '已定位到该问题属于当前页面的确定性缺陷，已在你的版本副本中直接修复。', 'v-1001-2', DATE_SUB(NOW(), INTERVAL 123 MINUTE), DATE_SUB(NOW(), INTERVAL 123 MINUTE)),
  ('p-1001-2', '1001', '林岚', 'visual', '首页卡片太挤了，希望导航和卡片都更紧凑一点。', 'accepted', '属于视觉 / 交互优化，已生成紧凑布局候选版本。', 'v-1001-2', DATE_SUB(NOW(), INTERVAL 2 HOUR), DATE_SUB(NOW(), INTERVAL 2 HOUR)),
  ('p-1002-1', '1002', '陈墨', 'visual', '首页首屏输入框不够聚焦，希望打开就能直接创作。', 'queued', '属于视觉 / 交互优化，已生成聚焦创作候选版本，请预览确认。', 'v-1002-1', DATE_SUB(NOW(), INTERVAL 5 HOUR), DATE_SUB(NOW(), INTERVAL 5 HOUR)),
  ('p-1003-1', '1003', '周野', 'visual', '希望在创作输入下面增加几个快捷操作入口。', 'accepted', '快捷入口不改变架构，已生成候选版本。', 'v-1003-1', DATE_SUB(NOW(), INTERVAL 26 HOUR), DATE_SUB(NOW(), INTERVAL 26 HOUR)),
  ('p-1004-1', '1004', '苏晴', 'feature', '希望支持多人同时编辑同一个项目。', 'reviewing', '这个提议涉及功能或架构改动，我不会直接生成。需要先确认权限模型、冲突解决和数据契约。', '', DATE_SUB(NOW(), INTERVAL 3 HOUR), DATE_SUB(NOW(), INTERVAL 3 HOUR));

INSERT INTO `evolution_message`
  (`public_id`, `user_id`, `user_name`, `role`, `content`, `created_at`)
VALUES
  ('c-1001-1', '1001', '林岚', 'user', '首页卡片太挤了。', DATE_SUB(NOW(), INTERVAL 126 MINUTE)),
  ('c-1001-2', '1001', '集思进化助手', 'agent', '属于视觉 / 交互优化，已生成紧凑布局候选版本，请预览。', DATE_SUB(NOW(), INTERVAL 125 MINUTE)),
  ('c-1001-3', '1001', '林岚', 'user', '删除按钮点了没反应，这个也要修。', DATE_SUB(NOW(), INTERVAL 122 MINUTE)),
  ('c-1001-4', '1001', '集思进化助手', 'agent', '已定位为确定性缺陷，在你的版本副本中直接修复，官方版本不受影响。', DATE_SUB(NOW(), INTERVAL 121 MINUTE)),
  ('c-1002-1', '1002', '陈墨', 'user', '打开首页后视线太分散，想直接开始写需求。', DATE_SUB(NOW(), INTERVAL 5 HOUR)),
  ('c-1002-2', '1002', '集思进化助手', 'agent', '我会把创作输入提到视觉中心，同时保留发现区入口，先给你看候选效果。', DATE_SUB(NOW(), INTERVAL 299 MINUTE)),
  ('c-1003-1', '1003', '周野', 'user', '能加几个快捷操作吗？', DATE_SUB(NOW(), INTERVAL 27 HOUR)),
  ('c-1003-2', '1003', '集思进化助手', 'agent', '快捷入口属于页面交互范围，已生成候选版本。', DATE_SUB(NOW(), INTERVAL 26 HOUR)),
  ('c-1004-1', '1004', '苏晴', 'user', '希望支持多人同时编辑同一个项目。', DATE_SUB(NOW(), INTERVAL 3 HOUR)),
  ('c-1004-2', '1004', '集思进化助手', 'agent', '这个提议涉及架构改动，需要先确认权限与冲突处理，暂不直接生成。', DATE_SUB(NOW(), INTERVAL 179 MINUTE));

INSERT INTO `evolution_task`
  (`public_id`, `version_public_id`, `user_id`, `user_name`, `version_label`, `origin_version`, `chat_summary`, `status`, `created_at`, `updated_at`)
VALUES
  ('t-seed-1', 'v-1003-1', '1003', '周野', 'v1003.1', 'v1.2026.11', '创作输入下增加快捷操作入口。 对话摘要：能加几个快捷操作吗？ / 快捷入口属于页面交互范围，已生成候选版本。', 'ci', DATE_SUB(NOW(), INTERVAL 20 HOUR), DATE_SUB(NOW(), INTERVAL 20 HOUR));

-- +goose Down

DROP TABLE IF EXISTS `evolution_task`;
DROP TABLE IF EXISTS `evolution_message`;
DROP TABLE IF EXISTS `evolution_proposal`;
DROP TABLE IF EXISTS `evolution_version`;

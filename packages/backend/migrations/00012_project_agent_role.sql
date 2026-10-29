-- project multi-role selection

-- +goose Up

CREATE TABLE `project_agent_role` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `project_id` bigint UNSIGNED NOT NULL COMMENT 'project 内部 ID',
  `agent_role_id` bigint UNSIGNED NOT NULL COMMENT 'agent_role ID',
  `sort_order` int NOT NULL DEFAULT 0 COMMENT '展示顺序',
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_project_agent_role` (`project_id`, `agent_role_id`),
  KEY `idx_project_agent_role_project` (`project_id`, `sort_order`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='项目 Agent 角色关联';

-- +goose Down

DROP TABLE IF EXISTS `project_agent_role`;

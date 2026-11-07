-- project leases for multi-replica sandbox ownership

-- +goose Up

CREATE TABLE `project_leases` (
  `project_id` bigint UNSIGNED NOT NULL COMMENT 'project.id',
  `sandbox_id` bigint UNSIGNED NOT NULL COMMENT 'sandbox_instance.id',
  `lease_until` datetime(3) NOT NULL COMMENT '租约到期时间',
  `epoch` bigint UNSIGNED NOT NULL DEFAULT 0 COMMENT 'owner 代数',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`project_id`),
  KEY `idx_project_leases_sandbox` (`sandbox_id`),
  KEY `idx_project_leases_until` (`lease_until`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='项目租约';

-- +goose Down

DROP TABLE IF EXISTS `project_leases`;

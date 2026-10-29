-- sandbox instance registry and project sticky binding

-- +goose Up

CREATE TABLE `sandbox_instance` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `public_id` char(36) NOT NULL COMMENT '对外 UUID',
  `name` varchar(128) NOT NULL COMMENT 'sandbox 实例唯一名称',
  `scheme` varchar(8) NOT NULL DEFAULT 'http' COMMENT '访问协议',
  `host` varchar(255) NOT NULL COMMENT '可访问主机/IP',
  `port` int UNSIGNED NOT NULL COMMENT '访问端口',
  `status` varchar(16) NOT NULL DEFAULT 'offline' COMMENT 'online/offline',
  `health_status` varchar(16) NOT NULL DEFAULT 'unknown' COMMENT 'unknown/healthy/unhealthy',
  `active_requests` int UNSIGNED NOT NULL DEFAULT 0 COMMENT '当前并发请求数',
  `max_concurrency` int UNSIGNED NOT NULL DEFAULT 8 COMMENT '最大并发请求数',
  `last_heartbeat_at` datetime(3) DEFAULT NULL COMMENT '最后心跳时间',
  `last_health_at` datetime(3) DEFAULT NULL COMMENT '最后健康检查时间',
  `health_error` varchar(255) NOT NULL DEFAULT '' COMMENT '最近健康检查错误',
  `last_selected_at` datetime(3) DEFAULT NULL COMMENT '最后被调度选择时间',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` datetime DEFAULT NULL COMMENT 'gorm 软删除时间',
  `extra_p` json DEFAULT NULL COMMENT '扩展字段',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_sandbox_instance_public_id` (`public_id`),
  UNIQUE KEY `uk_sandbox_instance_name` (`name`),
  KEY `idx_sandbox_instance_status` (`status`, `health_status`, `deleted_at`),
  KEY `idx_sandbox_instance_heartbeat` (`last_heartbeat_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='sandbox 实例注册表';

ALTER TABLE `project`
  ADD COLUMN `sandbox_id` bigint UNSIGNED DEFAULT NULL COMMENT '粘性调度的 sandbox 实例ID' AFTER `local_path`,
  ADD KEY `idx_project_sandbox_id` (`sandbox_id`);

-- +goose Down

ALTER TABLE `project`
  DROP INDEX `idx_project_sandbox_id`,
  DROP COLUMN `sandbox_id`;

DROP TABLE IF EXISTS `sandbox_instance`;

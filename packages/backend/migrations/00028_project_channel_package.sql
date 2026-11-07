-- +goose Up

CREATE TABLE `project_channel_package` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `public_id` char(36) NOT NULL COMMENT '渠道包公开ID',
  `project_public_id` char(36) NOT NULL COMMENT '项目公开ID',
  `workspace_id` bigint UNSIGNED NOT NULL COMMENT '工作区ID',
  `source_type` varchar(16) NOT NULL COMMENT '来源：html5/engine_artifact',
  `source_resource_id` char(36) DEFAULT NULL COMMENT '引擎成品资源ID',
  `channel` varchar(32) NOT NULL COMMENT '渠道',
  `status` varchar(16) NOT NULL COMMENT 'building/ready/failed',
  `version` varchar(64) NOT NULL DEFAULT '' COMMENT '来源版本',
  `file_path` varchar(512) NOT NULL DEFAULT '' COMMENT '相对产物根目录路径',
  `file_size_bytes` bigint UNSIGNED NOT NULL DEFAULT 0 COMMENT '产物大小',
  `manifest` json DEFAULT NULL COMMENT 'package-manifest 摘要',
  `error_message` varchar(1024) NOT NULL DEFAULT '' COMMENT '失败原因',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` datetime DEFAULT NULL COMMENT 'gorm 软删除时间',
  `extra_p` json DEFAULT NULL COMMENT '扩展字段',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_project_channel_package_public` (`public_id`),
  KEY `idx_project_channel_package_project` (`project_public_id`, `channel`, `created_at`),
  KEY `idx_project_channel_package_status` (`status`, `updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='项目渠道发行包';

-- +goose Down

DROP TABLE IF EXISTS `project_channel_package`;

-- clone capability: upload purpose and clone source/assets

-- +goose Up

ALTER TABLE `project_resource`
  ADD COLUMN `upload_purpose` varchar(32) NOT NULL DEFAULT '' COMMENT '上传用途：空/default/clone_image/clone_unit_test/clone_docs' AFTER `kind`;

CREATE TABLE `project_clone_source` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `project_public_id` char(36) NOT NULL COMMENT '项目公开ID',
  `source_url` varchar(1024) NOT NULL DEFAULT '' COMMENT '目标URL，仅供参考',
  `status` varchar(16) NOT NULL DEFAULT 'ready' COMMENT 'ready/disabled',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` datetime DEFAULT NULL COMMENT 'gorm 软删除时间',
  `extra_p` json DEFAULT NULL COMMENT '扩展字段',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_project_clone_source_project` (`project_public_id`),
  KEY `idx_project_clone_source_status` (`status`, `updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='克隆项目来源';

CREATE TABLE `project_clone_asset` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `project_public_id` char(36) NOT NULL COMMENT '项目公开ID',
  `resource_id` char(36) NOT NULL COMMENT '项目资源ID',
  `role` varchar(16) NOT NULL COMMENT 'cover/gallery/unit_test/docs',
  `sort_order` int NOT NULL DEFAULT 0 COMMENT '排序',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` datetime DEFAULT NULL COMMENT 'gorm 软删除时间',
  `extra_p` json DEFAULT NULL COMMENT '扩展字段',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_project_clone_asset_resource` (`project_public_id`, `resource_id`),
  KEY `idx_project_clone_asset_role` (`project_public_id`, `role`, `sort_order`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='克隆项目附件映射';

-- +goose Down

DROP TABLE IF EXISTS `project_clone_asset`;
DROP TABLE IF EXISTS `project_clone_source`;
ALTER TABLE `project_resource` DROP COLUMN `upload_purpose`;

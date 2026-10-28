-- workspace folders: logical subdirectories under a workspace

-- +goose Up

CREATE TABLE `workspace_folder` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `workspace_id` bigint UNSIGNED NOT NULL COMMENT '所属工作区ID',
  `parent_id` bigint UNSIGNED NOT NULL DEFAULT 0 COMMENT '父文件夹ID；0=工作区根目录',
  `name` varchar(64) NOT NULL COMMENT '文件夹名称',
  `created_by` bigint UNSIGNED NOT NULL COMMENT '创建者用户ID',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` datetime DEFAULT NULL COMMENT 'gorm 软删除时间',
  `extra_p` json DEFAULT NULL COMMENT '扩展字段',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_workspace_folder_name` (`workspace_id`, `parent_id`, `name`),
  KEY `idx_workspace_folder_parent` (`workspace_id`, `parent_id`, `deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='工作区文件夹表';

-- +goose Down

DROP TABLE IF EXISTS `workspace_folder`;

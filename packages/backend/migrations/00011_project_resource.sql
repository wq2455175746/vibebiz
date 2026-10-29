-- project knowledge resources

-- +goose Up

CREATE TABLE `project_resource` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `resource_id` char(36) NOT NULL COMMENT '对外资源 UUID',
  `project_public_id` char(36) DEFAULT NULL COMMENT '项目 UUID；创建项目前为空',
  `workspace_id` bigint UNSIGNED NOT NULL COMMENT '工作区ID',
  `user_id` bigint UNSIGNED NOT NULL COMMENT '上传用户ID',
  `original_name` varchar(255) NOT NULL COMMENT '用户原始文件名',
  `display_name` varchar(255) NOT NULL COMMENT '项目内唯一展示名',
  `storage_name` varchar(128) NOT NULL COMMENT 'S3/工作区存储名',
  `content_type` varchar(128) NOT NULL COMMENT 'MIME',
  `size_bytes` bigint UNSIGNED NOT NULL DEFAULT 0 COMMENT '文件大小',
  `kind` varchar(16) NOT NULL COMMENT 'document/text/image',
  `s3_staging_key` varchar(1024) NOT NULL DEFAULT '' COMMENT 'staging key',
  `s3_key` varchar(1024) NOT NULL DEFAULT '' COMMENT '最终 key',
  `status` varchar(16) NOT NULL DEFAULT 'staging' COMMENT 'staging/uploaded/ready/failed/deleted',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` datetime DEFAULT NULL COMMENT 'gorm 软删除时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_project_resource_id` (`resource_id`),
  KEY `idx_project_resource_project` (`project_public_id`, `display_name`),
  KEY `idx_project_resource_staging` (`workspace_id`, `user_id`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='项目知识库资源';

-- +goose Down

DROP TABLE IF EXISTS `project_resource`;

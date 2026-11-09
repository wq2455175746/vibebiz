-- +goose Up

CREATE TABLE `business_card` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `public_id` char(36) NOT NULL COMMENT '名片公开ID',
  `user_id` bigint UNSIGNED NOT NULL COMMENT '所属用户ID',
  `headline` varchar(80) NOT NULL DEFAULT '' COMMENT '一句话简介',
  `tags` json DEFAULT NULL COMMENT '标签数组',
  `featured_type` varchar(16) NOT NULL DEFAULT '' COMMENT 'project/image/pdf',
  `featured_project_public_id` char(36) NOT NULL DEFAULT '' COMMENT '项目主预览',
  `featured_s3_key` varchar(512) NOT NULL DEFAULT '' COMMENT '文件主预览S3 key',
  `featured_file_name` varchar(255) NOT NULL DEFAULT '' COMMENT '原始文件名',
  `featured_file_size` bigint UNSIGNED NOT NULL DEFAULT 0 COMMENT '文件大小',
  `published` tinyint UNSIGNED NOT NULL DEFAULT 0 COMMENT '是否已发布',
  `published_at` datetime DEFAULT NULL COMMENT '发布时间',
  `comment_count` int UNSIGNED NOT NULL DEFAULT 0 COMMENT '评论数',
  `is_hidden` tinyint UNSIGNED NOT NULL DEFAULT 0 COMMENT 'ops隐藏',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` datetime DEFAULT NULL COMMENT 'gorm 软删除时间',
  `extra_p` json DEFAULT NULL COMMENT '扩展字段',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_business_card_public` (`public_id`),
  UNIQUE KEY `uk_business_card_user` (`user_id`),
  KEY `idx_business_card_discover` (`published`, `is_hidden`, `updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户名片';

CREATE TABLE `business_card_item` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `card_id` bigint UNSIGNED NOT NULL COMMENT '名片ID',
  `project_public_id` char(36) NOT NULL COMMENT '项目公开ID',
  `sort_order` int NOT NULL DEFAULT 0 COMMENT '展示顺序',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` datetime DEFAULT NULL COMMENT 'gorm 软删除时间',
  `extra_p` json DEFAULT NULL COMMENT '扩展字段',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_business_card_item` (`card_id`, `project_public_id`),
  KEY `idx_business_card_item_project` (`project_public_id`),
  KEY `idx_business_card_item_order` (`card_id`, `sort_order`),
  CONSTRAINT `fk_business_card_item_card` FOREIGN KEY (`card_id`) REFERENCES `business_card` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='名片项目条目';

-- +goose Down

DROP TABLE IF EXISTS `business_card_item`;
DROP TABLE IF EXISTS `business_card`;

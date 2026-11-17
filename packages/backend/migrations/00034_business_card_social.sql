-- +goose Up

CREATE TABLE `business_card_comment` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `public_id` char(36) NOT NULL COMMENT '评论公开ID',
  `card_id` bigint UNSIGNED NOT NULL COMMENT '名片ID',
  `user_id` bigint UNSIGNED NOT NULL COMMENT '评论用户ID',
  `parent_id` bigint UNSIGNED NOT NULL DEFAULT 0 COMMENT '父评论ID，0为根评论',
  `root_id` bigint UNSIGNED NOT NULL DEFAULT 0 COMMENT '根评论ID，0表示自身为根',
  `depth` tinyint UNSIGNED NOT NULL DEFAULT 1 COMMENT '展示深度，最大3',
  `content` varchar(500) NOT NULL COMMENT '纯文本评论内容',
  `status` tinyint UNSIGNED NOT NULL DEFAULT 1 COMMENT '1正常，2已删除占位',
  `edited_at` datetime DEFAULT NULL COMMENT '编辑时间',
  `reply_count` int UNSIGNED NOT NULL DEFAULT 0 COMMENT '根评论回复总数',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` datetime DEFAULT NULL COMMENT 'gorm 软删除时间',
  `extra_p` json DEFAULT NULL COMMENT '扩展字段',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_business_card_comment_public` (`public_id`),
  KEY `idx_business_card_comment_roots` (`card_id`, `parent_id`, `id`),
  KEY `idx_business_card_comment_replies` (`root_id`, `id`),
  KEY `idx_business_card_comment_user` (`user_id`, `id`),
  CONSTRAINT `fk_business_card_comment_card` FOREIGN KEY (`card_id`) REFERENCES `business_card` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='名片评论';

CREATE TABLE `business_card_like` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `card_id` bigint UNSIGNED NOT NULL COMMENT '名片ID',
  `user_id` bigint UNSIGNED NOT NULL COMMENT '用户ID',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` datetime DEFAULT NULL COMMENT 'gorm 软删除时间',
  `extra_p` json DEFAULT NULL COMMENT '扩展字段',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_business_card_like_card_user` (`card_id`, `user_id`),
  KEY `idx_business_card_like_user` (`user_id`, `created_at`),
  CONSTRAINT `fk_business_card_like_card` FOREIGN KEY (`card_id`) REFERENCES `business_card` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='名片点赞';

ALTER TABLE `business_card`
  ADD COLUMN `like_count` int UNSIGNED NOT NULL DEFAULT 0 COMMENT '点赞数' AFTER `comment_count`;

-- +goose Down

ALTER TABLE `business_card` DROP COLUMN `like_count`;

DROP TABLE IF EXISTS `business_card_like`;
DROP TABLE IF EXISTS `business_card_comment`;

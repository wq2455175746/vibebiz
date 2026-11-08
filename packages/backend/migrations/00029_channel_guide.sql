-- channel guide content managed by ops-web

-- +goose Up

CREATE TABLE `channel_guide` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `channel` varchar(32) NOT NULL COMMENT '渠道标识',
  `title` varchar(64) NOT NULL COMMENT '渠道名称',
  `summary` varchar(255) NOT NULL DEFAULT '' COMMENT '渠道简介',
  `markdown` mediumtext NOT NULL COMMENT '指引 markdown',
  `steps` json DEFAULT NULL COMMENT '发布步骤',
  `requirements` json DEFAULT NULL COMMENT '资质要求',
  `enabled` tinyint(1) NOT NULL DEFAULT 1 COMMENT '是否在前台展示',
  `builtin` tinyint(1) NOT NULL DEFAULT 0 COMMENT '是否内置渠道',
  `sort_order` int NOT NULL DEFAULT 100 COMMENT '排序，越小越靠前',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_channel_guide_channel` (`channel`),
  KEY `idx_channel_guide_enabled_sort` (`enabled`, `sort_order`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='发布渠道指引';

-- +goose Down

DROP TABLE IF EXISTS `channel_guide`;

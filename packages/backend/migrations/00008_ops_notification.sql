-- ops notifications: version releases and inbox announcements

-- +goose Up

CREATE TABLE `ops_notification` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `title` varchar(200) NOT NULL COMMENT '消息标题',
  `content` mediumtext NOT NULL COMMENT 'Markdown 正文',
  `type` varchar(32) NOT NULL COMMENT '消息类型：version_release/inbox',
  `version` varchar(64) NOT NULL DEFAULT '' COMMENT '版本号',
  `link` varchar(1024) NOT NULL DEFAULT '' COMMENT '跳转链接',
  `status` varchar(24) NOT NULL DEFAULT 'draft' COMMENT '状态：draft/published/unpublished',
  `expire_at` datetime NOT NULL COMMENT '消息过期时间',
  `published_at` datetime DEFAULT NULL COMMENT '最近发布时间',
  `created_by` bigint UNSIGNED NOT NULL COMMENT '创建人 ops_user.id',
  `created_by_name` varchar(64) NOT NULL COMMENT '创建人用户名快照',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` datetime DEFAULT NULL COMMENT 'gorm 软删除时间',
  `extra_p` json DEFAULT NULL COMMENT '扩展字段',
  PRIMARY KEY (`id`),
  KEY `idx_ops_notification_status` (`status`, `deleted_at`),
  KEY `idx_ops_notification_type` (`type`, `status`, `deleted_at`),
  KEY `idx_ops_notification_expire` (`expire_at`),
  KEY `idx_ops_notification_published` (`published_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='运营消息通知表';

-- +goose Down

DROP TABLE IF EXISTS `ops_notification`;

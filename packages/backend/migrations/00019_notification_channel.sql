-- personal notification channels

-- +goose Up

CREATE TABLE `notification_channel` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `user_id` bigint UNSIGNED NOT NULL COMMENT '所属用户ID',
  `name` varchar(64) NOT NULL COMMENT '渠道名称',
  `type` varchar(16) NOT NULL COMMENT 'webhook/feishu/wecom/dingtalk/email',
  `config_cipher` mediumtext NOT NULL COMMENT '加密后的渠道配置 JSON',
  `enabled` tinyint(1) NOT NULL DEFAULT 1 COMMENT '是否启用',
  `last_test_at` datetime DEFAULT NULL COMMENT '最近测试时间',
  `last_test_status` varchar(16) NOT NULL DEFAULT '' COMMENT '最近测试结果：success/failed',
  `last_test_error` varchar(500) NOT NULL DEFAULT '' COMMENT '最近测试错误（已脱敏）',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_notification_channel_user_name` (`user_id`, `name`),
  KEY `idx_notification_channel_user_enabled` (`user_id`, `enabled`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='个人通知渠道表';

-- +goose Down

DROP TABLE IF EXISTS `notification_channel`;

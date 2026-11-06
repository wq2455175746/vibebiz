-- user access trace log

-- +goose Up

CREATE TABLE `user_access_log` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `event_id` char(36) NOT NULL COMMENT '事件UUID，同步幂等键',
  `event_type` varchar(16) NOT NULL COMMENT '事件类型：login/request',
  `user_id` bigint UNSIGNED DEFAULT NULL COMMENT '用户id，登录失败未知用户时为空',
  `username` varchar(64) NOT NULL DEFAULT '' COMMENT '用户名快照',
  `workspace_id` bigint UNSIGNED NOT NULL DEFAULT 0 COMMENT '当前工作区id',
  `project_id` varchar(64) NOT NULL DEFAULT '' COMMENT '项目公开id',
  `ip` varchar(64) NOT NULL DEFAULT '' COMMENT '客户端IP',
  `region_country` varchar(64) NOT NULL DEFAULT '' COMMENT '国家，本期留空',
  `region_province` varchar(64) NOT NULL DEFAULT '' COMMENT '省份，本期留空',
  `region_city` varchar(64) NOT NULL DEFAULT '' COMMENT '城市，本期留空',
  `device_id` varchar(64) NOT NULL DEFAULT '' COMMENT '前端设备码',
  `device_type` varchar(32) NOT NULL DEFAULT '' COMMENT 'desktop/mobile/tablet/bot/other',
  `os` varchar(64) NOT NULL DEFAULT '' COMMENT '操作系统',
  `browser` varchar(64) NOT NULL DEFAULT '' COMMENT '浏览器',
  `user_agent` varchar(512) NOT NULL DEFAULT '' COMMENT 'User-Agent',
  `referer` varchar(1024) NOT NULL DEFAULT '' COMMENT '来源页',
  `request_method` varchar(16) NOT NULL DEFAULT '' COMMENT 'HTTP方法',
  `request_path` varchar(512) NOT NULL DEFAULT '' COMMENT '原始请求路径',
  `request_query` varchar(1024) NOT NULL DEFAULT '' COMMENT '脱敏后的query',
  `route` varchar(255) NOT NULL DEFAULT '' COMMENT '路由模板',
  `status_code` int NOT NULL DEFAULT 0 COMMENT '响应状态码',
  `duration_ms` int NOT NULL DEFAULT 0 COMMENT '耗时毫秒',
  `request_id` varchar(64) NOT NULL DEFAULT '' COMMENT '请求id',
  `trace_id` varchar(64) NOT NULL DEFAULT '' COMMENT 'trace id',
  `login_result` varchar(16) NOT NULL DEFAULT '' COMMENT 'success/failed，仅登录事件',
  `error_message` varchar(255) NOT NULL DEFAULT '' COMMENT '登录失败原因',
  `event_at` datetime(3) NOT NULL COMMENT '事件发生时间（毫秒）',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '入库时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_access_log_event_id` (`event_id`),
  KEY `idx_user_access_log_event_at` (`event_at`),
  KEY `idx_user_access_log_user_time` (`user_id`, `event_at`),
  KEY `idx_user_access_log_type_time` (`event_type`, `event_at`),
  KEY `idx_user_access_log_ip` (`ip`),
  KEY `idx_user_access_log_device` (`device_id`),
  KEY `idx_user_access_log_project` (`project_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户访问痕迹日志表';

-- +goose Down

DROP TABLE IF EXISTS `user_access_log`;

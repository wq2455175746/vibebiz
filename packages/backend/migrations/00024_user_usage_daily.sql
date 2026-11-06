-- +goose Up

ALTER TABLE `user_quota`
  ADD COLUMN `project_daily_quota` int UNSIGNED NOT NULL DEFAULT 0 COMMENT '每日项目额度，0 表示不限',
  ADD COLUMN `llm_token_daily_quota` bigint UNSIGNED NOT NULL DEFAULT 0 COMMENT '每日 Token 额度，0 表示不限',
  ADD COLUMN `upload_daily_count_quota` int UNSIGNED NOT NULL DEFAULT 0 COMMENT '每日上传文件数额度，0 表示不限',
  ADD COLUMN `upload_daily_bytes_quota` bigint UNSIGNED NOT NULL DEFAULT 0 COMMENT '每日上传字节额度，0 表示不限';

CREATE TABLE `user_usage_daily` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `user_id` bigint UNSIGNED NOT NULL COMMENT '用户id',
  `usage_date` date NOT NULL COMMENT '自然日（USAGE_TIMEZONE）',
  `projects_created` int UNSIGNED NOT NULL DEFAULT 0 COMMENT '当日创建项目数',
  `input_tokens` bigint UNSIGNED NOT NULL DEFAULT 0 COMMENT '当日输入Token',
  `output_tokens` bigint UNSIGNED NOT NULL DEFAULT 0 COMMENT '当日输出Token',
  `model_requests` bigint UNSIGNED NOT NULL DEFAULT 0 COMMENT '当日底层模型请求次数',
  `upload_count` int UNSIGNED NOT NULL DEFAULT 0 COMMENT '当日上传文件数',
  `upload_bytes` bigint UNSIGNED NOT NULL DEFAULT 0 COMMENT '当日上传字节数',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '入库时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_usage_daily_user_date` (`user_id`, `usage_date`),
  KEY `idx_user_usage_daily_date` (`usage_date`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户每日使用指标';

INSERT INTO `user_usage_daily` (`user_id`, `usage_date`, `projects_created`)
SELECT `user_id`, DATE(`created_at`), COUNT(*)
FROM `project`
WHERE `deleted_at` IS NULL
GROUP BY `user_id`, DATE(`created_at`)
ON DUPLICATE KEY UPDATE `projects_created` = VALUES(`projects_created`);

INSERT INTO `user_usage_daily` (`user_id`, `usage_date`, `upload_count`, `upload_bytes`)
SELECT `user_id`, DATE(`created_at`), COUNT(*), COALESCE(SUM(`size_bytes`), 0)
FROM `project_resource`
WHERE `deleted_at` IS NULL AND `status` = 'ready'
GROUP BY `user_id`, DATE(`created_at`)
ON DUPLICATE KEY UPDATE
  `upload_count` = VALUES(`upload_count`),
  `upload_bytes` = VALUES(`upload_bytes`);

-- +goose Down

DROP TABLE IF EXISTS `user_usage_daily`;

ALTER TABLE `user_quota`
  DROP COLUMN `project_daily_quota`,
  DROP COLUMN `llm_token_daily_quota`,
  DROP COLUMN `upload_daily_count_quota`,
  DROP COLUMN `upload_daily_bytes_quota`;

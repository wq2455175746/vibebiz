-- admin user controls and hidden projects

-- +goose Up

ALTER TABLE `user`
  ADD COLUMN `disabled_reason` varchar(500) NOT NULL DEFAULT '' COMMENT '最近一次禁用原因' AFTER `status`,
  ADD COLUMN `disabled_at` datetime DEFAULT NULL COMMENT '最近一次禁用时间' AFTER `disabled_reason`,
  ADD COLUMN `disabled_by` varchar(255) NOT NULL DEFAULT '' COMMENT '最近一次禁用操作人' AFTER `disabled_at`;

ALTER TABLE `project`
  ADD COLUMN `is_hidden` tinyint UNSIGNED NOT NULL DEFAULT 0 COMMENT '是否隐藏：1隐藏，0正常' AFTER `status`,
  ADD KEY `idx_project_hidden` (`is_hidden`, `deleted_at`);

-- +goose Down

ALTER TABLE `project`
  DROP KEY `idx_project_hidden`,
  DROP COLUMN `is_hidden`;

ALTER TABLE `user`
  DROP COLUMN `disabled_by`,
  DROP COLUMN `disabled_at`,
  DROP COLUMN `disabled_reason`;

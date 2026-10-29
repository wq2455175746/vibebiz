-- +goose Up

ALTER TABLE `project`
  ADD COLUMN `public_id` char(36) NULL AFTER `id`,
  ADD COLUMN `prompt` mediumtext NULL AFTER `name`;

UPDATE `project`
SET `public_id` = UUID()
WHERE `public_id` IS NULL OR `public_id` = '';

UPDATE `project`
SET `prompt` = ''
WHERE `prompt` IS NULL;

ALTER TABLE `project`
  MODIFY COLUMN `public_id` char(36) NOT NULL COMMENT '对外 UUID',
  MODIFY COLUMN `prompt` mediumtext NOT NULL COMMENT '用户创建项目时的需求',
  MODIFY COLUMN `status` varchar(16) NOT NULL DEFAULT 'ready' COMMENT '项目状态：ready/error；预留 draft/active/archived',
  ADD UNIQUE KEY `uk_project_public_id` (`public_id`);

-- +goose Down

ALTER TABLE `project`
  DROP INDEX `uk_project_public_id`,
  DROP COLUMN `public_id`,
  DROP COLUMN `prompt`;

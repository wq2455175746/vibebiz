-- enterprise project mode

-- +goose Up

ALTER TABLE `project`
  ADD COLUMN `mode` varchar(16) NOT NULL DEFAULT 'standard' COMMENT '项目模式：standard/enterprise' AFTER `prompt`;

-- +goose Down

ALTER TABLE `project`
  DROP COLUMN `mode`;

-- project preview entry

-- +goose Up

ALTER TABLE `project`
  ADD COLUMN `preview_path` varchar(512) NOT NULL DEFAULT '' COMMENT '预览入口相对路径，空值表示 index.html' AFTER `mode`;

-- +goose Down

ALTER TABLE `project`
  DROP COLUMN `preview_path`;

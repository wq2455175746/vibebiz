-- project cover image

-- +goose Up

ALTER TABLE `project`
  ADD COLUMN `cover_s3_path` varchar(1024) DEFAULT NULL COMMENT '项目简介图 S3 路径' AFTER `s3_path`;

-- +goose Down

ALTER TABLE `project`
  DROP COLUMN `cover_s3_path`;

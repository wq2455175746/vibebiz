-- settings: workspace avatars and default storage quota

-- +goose Up

ALTER TABLE `workspace`
  ADD COLUMN `avatar_url` varchar(1024) DEFAULT NULL COMMENT '头像 URL' AFTER `description`,
  ADD COLUMN `avatar_s3_path` varchar(1024) DEFAULT NULL COMMENT '头像 S3 对象路径' AFTER `avatar_url`;

UPDATE `user_quota` q
JOIN `user` u ON u.id = q.user_id
SET q.storage_quota_bytes = 10737418240
WHERE u.username IN ('wanghg11', 'vistor');

-- +goose Down

UPDATE `user_quota` q
JOIN `user` u ON u.id = q.user_id
SET q.storage_quota_bytes = 0
WHERE u.username IN ('wanghg11', 'vistor');

ALTER TABLE `workspace`
  DROP COLUMN `avatar_s3_path`,
  DROP COLUMN `avatar_url`;

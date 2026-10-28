-- seed default local users, ops admin, personal workspaces and quotas

-- +goose Up

INSERT INTO `user` (`email`, `source`, `username`, `password_hash`, `nickname`, `status`, `email_verified_at`)
SELECT '2455175746@qq.com', 'CUSTOM', 'wanghg11', '$2a$12$teiprG6aKjqzfY.rBMKCVOdCX9jtWzvOwdbrr0W5FyzqKIu9qcveq', 'wanghg11', 1, NOW() FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `user` WHERE `username` = 'wanghg11' OR `email` = '2455175746@qq.com');

INSERT INTO `user` (`email`, `source`, `username`, `password_hash`, `nickname`, `status`, `email_verified_at`)
SELECT 'vistor_vibebiz@qq.com', 'CUSTOM', 'vistor', '$2a$12$o6NLzqC2jK.GDPA/wq9jRen9T6sjqiHNa41f8TZBEBnjcMgAFcS7e', 'vistor', 1, NOW() FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `user` WHERE `username` = 'vistor' OR `email` = 'vistor_vibebiz@qq.com');

INSERT INTO `ops_user` (`username`, `email`, `password_hash`, `nickname`, `role_code`, `source`, `status`, `created_by`, `updated_by`)
SELECT 'admin', 'admin_vibebiz@qq.com', '$2a$12$9nHY8OcLnd2RgzJB9Nt4R.S6IwqVnrI6Tkzkksla9Es1Vi046APVK', 'admin', 'super_admin', 'local', 1, 'seed', 'seed' FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM `ops_user` WHERE `username` = 'admin' OR `email` = 'admin_vibebiz@qq.com');

INSERT INTO `workspace` (`owner_user_id`, `name`, `description`, `workspace_type`, `visibility`, `status`, `created_by`)
SELECT u.id, CONCAT(u.username, ' 的个人工作区'), '默认个人工作区', 'personal', 'private', 1, 'seed'
FROM `user` u
WHERE u.username = 'wanghg11'
  AND NOT EXISTS (
    SELECT 1 FROM `workspace` w
    WHERE w.owner_user_id = u.id
      AND w.name = CONCAT(u.username, ' 的个人工作区')
      AND w.deleted_at IS NULL
  );

INSERT INTO `workspace` (`owner_user_id`, `name`, `description`, `workspace_type`, `visibility`, `status`, `created_by`)
SELECT u.id, CONCAT(u.username, ' 的个人工作区'), '默认个人工作区', 'personal', 'private', 1, 'seed'
FROM `user` u
WHERE u.username = 'vistor'
  AND NOT EXISTS (
    SELECT 1 FROM `workspace` w
    WHERE w.owner_user_id = u.id
      AND w.name = CONCAT(u.username, ' 的个人工作区')
      AND w.deleted_at IS NULL
  );

INSERT INTO `workspace_user` (`workspace_id`, `user_id`, `role`, `status`, `joined_at`)
SELECT w.id, u.id, 'owner', 1, NOW()
FROM `workspace` w
JOIN `user` u ON u.id = w.owner_user_id
WHERE u.username IN ('wanghg11', 'vistor')
  AND NOT EXISTS (
    SELECT 1 FROM `workspace_user` wu
    WHERE wu.workspace_id = w.id AND wu.user_id = u.id
  );

INSERT INTO `user_quota` (`user_id`, `plan_code`, `status`)
SELECT u.id, 'free', 1
FROM `user` u
WHERE u.username IN ('wanghg11', 'vistor')
  AND NOT EXISTS (SELECT 1 FROM `user_quota` q WHERE q.user_id = u.id);

-- +goose Down

DELETE wu FROM `workspace_user` wu
JOIN `user` u ON u.id = wu.user_id
WHERE u.username IN ('wanghg11', 'vistor');

DELETE w FROM `workspace` w
JOIN `user` u ON u.id = w.owner_user_id
WHERE u.username IN ('wanghg11', 'vistor');

DELETE q FROM `user_quota` q
JOIN `user` u ON u.id = q.user_id
WHERE u.username IN ('wanghg11', 'vistor');

DELETE FROM `ops_user` WHERE `username` = 'admin' AND `email` = 'admin_vibebiz@qq.com';

DELETE FROM `user` WHERE `username` IN ('wanghg11', 'vistor');

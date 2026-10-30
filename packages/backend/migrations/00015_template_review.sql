-- template capability, project review and targeted notifications

-- +goose Up

ALTER TABLE `project`
  ADD COLUMN `review_status` varchar(16) NOT NULL DEFAULT 'pending' COMMENT '审核状态：pending/approved/rejected' AFTER `status`,
  ADD COLUMN `review_note` varchar(1000) NOT NULL DEFAULT '' COMMENT '审核备注' AFTER `review_status`,
  ADD COLUMN `reviewed_by` varchar(64) NOT NULL DEFAULT '' COMMENT '审核人' AFTER `review_note`,
  ADD COLUMN `reviewed_at` datetime DEFAULT NULL COMMENT '审核时间' AFTER `reviewed_by`;

ALTER TABLE `project_topic`
  ADD COLUMN `parent_id` bigint UNSIGNED DEFAULT NULL COMMENT '父类别ID' AFTER `id`,
  ADD KEY `idx_project_topic_parent` (`parent_id`);

INSERT INTO `project_topic`
  (`topic_code`, `topic_name`, `description`, `is_default`, `sort_order`)
VALUES
  ('cat_game', '游戏', '游戏类项目', 0, 5),
  ('cat_web', '网页', '网页与 Web 应用', 0, 6),
  ('cat_app', 'App', '应用类项目', 0, 7)
ON DUPLICATE KEY UPDATE `topic_name` = VALUES(`topic_name`), `description` = VALUES(`description`);

UPDATE `project_topic` AS child
JOIN `project_topic` AS parent ON parent.`topic_code` = 'cat_game'
SET child.`parent_id` = parent.`id`
WHERE child.`topic_code` = 'game';

UPDATE `project_topic` AS child
JOIN `project_topic` AS parent ON parent.`topic_code` = 'cat_web'
SET child.`parent_id` = parent.`id`
WHERE child.`topic_code` IN ('common', 'video', 'slides');

INSERT INTO `project_topic`
  (`topic_code`, `topic_name`, `description`, `parent_id`, `is_default`, `sort_order`)
SELECT 'app', 'App 应用', '移动端 App 项目', parent.`id`, 0, 50
FROM `project_topic` AS parent
WHERE parent.`topic_code` = 'cat_app'
ON DUPLICATE KEY UPDATE `topic_name` = VALUES(`topic_name`), `description` = VALUES(`description`);

ALTER TABLE `project_template`
  ADD COLUMN `source_project_id` bigint UNSIGNED DEFAULT NULL COMMENT '来源项目ID' AFTER `topic_id`,
  ADD COLUMN `recommend_score` int NOT NULL DEFAULT 0 COMMENT '推荐分数 0-100' AFTER `sort_order`,
  ADD KEY `idx_template_score` (`status`, `recommend_score`);

ALTER TABLE `ops_notification`
  ADD COLUMN `recipient_user_id` bigint UNSIGNED NOT NULL DEFAULT 0 COMMENT '接收用户ID，0=全局' AFTER `type`,
  ADD KEY `idx_ops_notification_recipient` (`recipient_user_id`, `status`, `deleted_at`);

-- +goose Down

ALTER TABLE `ops_notification`
  DROP KEY `idx_ops_notification_recipient`,
  DROP COLUMN `recipient_user_id`;

ALTER TABLE `project_template`
  DROP KEY `idx_template_score`,
  DROP COLUMN `recommend_score`,
  DROP COLUMN `source_project_id`;

UPDATE `project_topic` SET `parent_id` = NULL WHERE `topic_code` IN ('game', 'common', 'video', 'slides', 'app');
DELETE FROM `project_topic` WHERE `topic_code` IN ('cat_game', 'cat_web', 'cat_app', 'app');

ALTER TABLE `project_topic`
  DROP KEY `idx_project_topic_parent`,
  DROP COLUMN `parent_id`;

ALTER TABLE `project`
  DROP COLUMN `reviewed_at`,
  DROP COLUMN `reviewed_by`,
  DROP COLUMN `review_note`,
  DROP COLUMN `review_status`;

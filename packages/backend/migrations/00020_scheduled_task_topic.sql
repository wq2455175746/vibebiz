-- scheduled task topic seed

-- +goose Up

INSERT INTO `project_topic`
  (`topic_code`, `topic_name`, `description`, `prompt_template`, `is_default`, `sort_order`, `status`)
VALUES
  ('scheduled', '定时任务', '用户定时任务专用主题',
   '你是定时任务执行助手。你会收到任务描述和最近对话上下文。请先检查固定项目的当前文件与状态，再直接执行任务，不要向用户提问。完成后用简洁中文说明本次执行结果、修改的文件和需要用户注意的风险。',
   0, 90, 1)
ON DUPLICATE KEY UPDATE
  `topic_name` = VALUES(`topic_name`),
  `description` = VALUES(`description`),
  `prompt_template` = VALUES(`prompt_template`),
  `status` = VALUES(`status`);

-- +goose Down

DELETE FROM `project_topic` WHERE `topic_code` = 'scheduled';

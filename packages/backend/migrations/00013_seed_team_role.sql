-- built-in team role for multi-agent collaboration

-- +goose Up

INSERT INTO `agent_role`
  (`role_code`, `role_name_en`, `role_name_zh`, `description`, `system_prompt`, `is_builtin`, `sort_order`)
VALUES
  ('team', 'Team', '团队', '由全部内置角色协作完成复杂任务。', '你是团队协调者。先将用户任务拆分为可独立执行的子任务，分析依赖和风险，再按角色能力调用子 Agent；子 Agent 共享当前项目工作区，主 Agent 负责检查冲突、整合结果并验证最终交付。不要重复分配同一修改；无法确认时先读取现有代码再决策。', 1, 5)
ON DUPLICATE KEY UPDATE
  `role_name_en` = VALUES(`role_name_en`),
  `role_name_zh` = VALUES(`role_name_zh`),
  `description` = VALUES(`description`),
  `system_prompt` = VALUES(`system_prompt`),
  `is_builtin` = VALUES(`is_builtin`),
  `sort_order` = VALUES(`sort_order`);

-- +goose Down

DELETE FROM `agent_role` WHERE `role_code` = 'team';

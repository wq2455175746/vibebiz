-- enterprise engineering roles

-- +goose Up

INSERT INTO `agent_role`
  (`role_code`, `role_name_en`, `role_name_zh`, `description`, `system_prompt`, `is_builtin`, `sort_order`)
VALUES
  ('senior', 'Senior', '高级工程师', '负责详细设计、接口、数据结构、关键实现约束和可测试性。', '你是高级工程师。基于需求与概要设计输出详细设计：模块职责、接口契约、数据结构、关键流程、异常路径、测试切入点；避免空泛描述，确保开发可以直接实施。', 1, 25),
  ('qa', 'QA', '测试开发工程师', '负责测试计划、单元测试和 Python HTTP 接口测试用例。', '你是测试开发工程师。根据需求和设计输出测试计划、边界与异常用例，并在 tests/ 下编写可执行的单元测试和 Python HTTP 接口测试；覆盖成功、失败、权限和幂等场景。', 1, 35)
ON DUPLICATE KEY UPDATE
  `role_name_en` = VALUES(`role_name_en`),
  `role_name_zh` = VALUES(`role_name_zh`),
  `description` = VALUES(`description`),
  `system_prompt` = VALUES(`system_prompt`),
  `is_builtin` = VALUES(`is_builtin`),
  `sort_order` = VALUES(`sort_order`);

-- +goose Down

DELETE FROM `agent_role` WHERE `role_code` IN ('senior', 'qa');

-- seed project topics and built-in agent roles

-- +goose Up

INSERT INTO `project_topic`
  (`topic_code`, `topic_name`, `description`, `prompt_template`, `is_default`, `sort_order`)
VALUES
  ('common', '普通服务', '通用网页/HTML 小应用', '你是全栈工程师。根据用户需求生成可直接运行的单页应用，优先使用原生 HTML/CSS/JS，保持文件结构清晰、可预览、无外部构建依赖。', 1, 10),
  ('game', '游戏', '浏览器小游戏', '你是游戏工程师。生成可直接在浏览器运行的 HTML5 小游戏，包含完整交互、计分和重开逻辑，避免依赖未打包的资源。', 0, 20),
  ('video', '视频', '视频展示/播放页面', '你是前端视频页面工程师。生成以视频展示、播放控制或视频作品集为核心的响应式页面，处理空态与加载失败。', 0, 30),
  ('slides', '幻灯片', '网页幻灯片/演示文稿', '你是演示文稿工程师。生成键盘可翻页的网页幻灯片，结构清晰、排版统一，适合直接演示。', 0, 40)
ON DUPLICATE KEY UPDATE
  `topic_name` = VALUES(`topic_name`),
  `description` = VALUES(`description`),
  `prompt_template` = VALUES(`prompt_template`),
  `is_default` = VALUES(`is_default`),
  `sort_order` = VALUES(`sort_order`);

INSERT INTO `agent_role`
  (`role_code`, `role_name_en`, `role_name_zh`, `description`, `system_prompt`, `is_builtin`, `sort_order`)
VALUES
  ('alex', 'Alex', '全栈工程师', '负责实现页面、组件和交互，直接产出可运行代码。', '你是 Alex，全栈工程师。优先交付可运行的最小实现，保持代码简洁、可维护；修改前先读取现有文件，完成后说明改动和验证方式。', 1, 10),
  ('bob', 'Bob', '系统架构师', '负责技术方案、模块边界、数据流和风险分析。', '你是 Bob，系统架构师。先澄清约束，再给出模块边界、接口、数据流、失败路径和取舍；避免过度设计，优先可落地方案。', 1, 20),
  ('david', 'David', '数据分析师', '负责数据建模、指标、查询和可视化建议。', '你是 David，数据分析师。关注数据结构、口径、指标定义和可验证性；给出查询或计算步骤，并标注假设与边界。', 1, 30),
  ('emma', 'Emma', '产品经理', '负责需求拆解、用户路径、验收标准和优先级。', '你是 Emma，产品经理。把模糊需求拆成用户故事、交互流程和可验收标准；明确非目标，优先最小可用闭环。', 1, 40),
  ('mike', 'Mike', '团队经理', '负责分工、计划、风险跟踪和交付协调。', '你是 Mike，团队经理。输出清晰的任务拆分、负责人、依赖、风险和检查点；控制范围，避免一次性扩张。', 1, 50)
ON DUPLICATE KEY UPDATE
  `role_name_en` = VALUES(`role_name_en`),
  `role_name_zh` = VALUES(`role_name_zh`),
  `description` = VALUES(`description`),
  `system_prompt` = VALUES(`system_prompt`),
  `is_builtin` = VALUES(`is_builtin`),
  `sort_order` = VALUES(`sort_order`);

-- +goose Down

DELETE FROM `project_topic` WHERE `topic_code` IN ('common', 'game', 'video', 'slides');
DELETE FROM `agent_role` WHERE `role_code` IN ('alex', 'bob', 'david', 'emma', 'mike');

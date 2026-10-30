-- enterprise project stages

-- +goose Up

CREATE TABLE `project_stage` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `project_id` varchar(64) NOT NULL COMMENT '项目公开ID',
  `stage_key` varchar(32) NOT NULL COMMENT '阶段编码',
  `seq` int NOT NULL COMMENT '阶段顺序',
  `status` varchar(16) NOT NULL COMMENT 'locked/active/done/placeholder',
  `artifact_path` varchar(255) NOT NULL DEFAULT '' COMMENT '阶段固定产物路径',
  `started_at` datetime DEFAULT NULL COMMENT '开始时间',
  `completed_at` datetime DEFAULT NULL COMMENT '完成时间',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_project_stage` (`project_id`, `stage_key`),
  KEY `idx_project_stage_status` (`project_id`, `status`, `seq`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='企业项目阶段表';

-- +goose Down

DROP TABLE IF EXISTS `project_stage`;

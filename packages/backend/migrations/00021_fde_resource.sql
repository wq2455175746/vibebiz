-- FDE human resources managed by ops

-- +goose Up

CREATE TABLE `fde_resource` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `name` varchar(64) NOT NULL COMMENT '姓名',
  `contact` varchar(128) NOT NULL COMMENT '联系方式',
  `skills` json NOT NULL COMMENT '技能标签',
  `region` varchar(64) NOT NULL DEFAULT '' COMMENT '所在区域',
  `note` varchar(500) NOT NULL DEFAULT '' COMMENT '备注',
  `enabled` tinyint(1) NOT NULL DEFAULT 1 COMMENT '是否启用',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_fde_resource_enabled` (`enabled`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='FDE 人力资源表';

-- +goose Down

DROP TABLE IF EXISTS `fde_resource`;

-- +goose Up

DELETE FROM `evolution_task`
WHERE `public_id` = 't-seed-1';

DELETE FROM `evolution_message`
WHERE `public_id` IN (
  'c-1001-1', 'c-1001-2', 'c-1001-3', 'c-1001-4',
  'c-1002-1', 'c-1002-2',
  'c-1003-1', 'c-1003-2',
  'c-1004-1', 'c-1004-2'
);

DELETE FROM `evolution_proposal`
WHERE `public_id` IN ('p-1001-1', 'p-1001-2', 'p-1002-1', 'p-1003-1', 'p-1004-1');

DELETE FROM `evolution_version`
WHERE `public_id` IN ('v-1001-2', 'v-1001-1', 'v-1002-1', 'v-1003-1', 'v-1004-1');

-- +goose Down

SELECT 1;

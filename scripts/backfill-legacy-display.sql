-- Run only after migrations 006 and 007. Intended databases are explicit.
-- Copies only missing display values for identical imported identities.
-- Restores display values and legacy resource navigation metadata only.
-- It changes no account, grant, lifecycle, backend-service route, or timestamp fields.
START TRANSACTION;
SELECT id FROM kerthus_saas.platform_lock WHERE id = 1 FOR UPDATE;

UPDATE kerthus_saas.apps AS target
JOIN beehive_saas.d_app AS source
  ON target.id = source.id AND BINARY target.code = BINARY source.code
SET target.icon = IF(target.icon = '', COALESCE(source.icon, ''), target.icon),
    target.description = IF(target.description = '', COALESCE(source.`desc`, ''), target.description),
    target.remark = IF(target.remark = '', COALESCE(source.remark, ''), target.remark)
WHERE (target.icon = '' AND COALESCE(source.icon, '') <> '')
   OR (target.description = '' AND COALESCE(source.`desc`, '') <> '')
   OR (target.remark = '' AND COALESCE(source.remark, '') <> '');
SELECT ROW_COUNT() AS app_rows_backfilled;

UPDATE kerthus_saas.resources AS target
JOIN beehive_saas.d_app_resource AS source
  ON target.id = source.id AND target.app_id = source.app_id
 AND BINARY target.code = BINARY source.code
SET target.redirect = IF(target.redirect = '', COALESCE(source.redirect, ''), target.redirect),
    target.remark = IF(COALESCE(target.remark, '') = '', COALESCE(source.remark, ''), target.remark),
    target.open_with = IF(target.open_with = '', COALESCE(source.open_with, ''), target.open_with)
WHERE (target.redirect = '' AND COALESCE(source.redirect, '') <> '')
   OR (COALESCE(target.remark, '') = '' AND COALESCE(source.remark, '') <> '')
   OR (target.open_with = '' AND COALESCE(source.open_with, '') <> '');
SELECT ROW_COUNT() AS resource_rows_backfilled;
COMMIT;

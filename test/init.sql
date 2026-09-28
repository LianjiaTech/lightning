FLUSH LOGS;
SET GLOBAL BINLOG_FORMAT=ROW;
SET GLOBAL BINLOG_ROW_IMAGE=FULL;
SET GLOBAL ENFORCE_GTID_CONSISTENCY = ON;

-- Do not try to switch GTID_MODE here: it cannot be set inside a stored
-- procedure (ERROR 1838), and MySQL 8.0 defaults to OFF while 8.4+ defaults
-- to ON. The test suite uses position-based replication, so GTID mode is not
-- required. Enable it via server options if you need GTID-based sync.

SET GLOBAL binlog_rows_query_log_events=on;

ALTER USER 'root'@'%' IDENTIFIED BY '******';

FLUSH LOGS;

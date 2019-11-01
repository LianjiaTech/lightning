#!/bin/bash

DB=$1
TABLE=$2
INTERVAL=100

MYSQL_USER="root"
MYSQL_PASS='******'
MYSQL_PORT=3306
MYSQL_HOST="127.0.0.1"
MYSQL="mysql -A -u${MYSQL_USER} -p${MYSQL_PASS} -h${MYSQL_HOST} -P${MYSQL_PORT} --connect-timeout=5 "

${MYSQL} "${DB}" -NBe "CREATE TABLE _${TABLE}_new LIKE ${TABLE}"

MIN_MAX=$(${MYSQL} "${DB}" -NBe "SELECT MIN(id) min_id, MAX(id) max_id FROM ${TABLE}")

MIN_ID=$(echo "${MIN_MAX}" | awk '{print $1}')
MAX_ID=$(echo "${MIN_MAX}" | awk '{print $2}')

LAST_ID=${MIN_ID}
TOTAL_CHUNK=$(echo "${MAX_ID}/100 + 1" | bc)
CHUNK=1

while true; do
    ${MYSQL} "${DB}" -NBe "INSERT LOW_PRIORITY IGNORE INTO _${TABLE}_new SELECT * FROM ${TABLE} WHERE id >= ${LAST_ID} AND id < ${LAST_ID} + ${INTERVAL} LOCK IN SHARE MODE /*lightning coping table, ${CHUNK} of ${TOTAL_CHUNK} */" 
    LAST_ID=$(echo "${LAST_ID} + 100" | bc)
    if [ ${LAST_ID} -gt ${MAX_ID} ]; then
        break
    else
        CHUNK=$(echo "${CHUNK} + 1" | bc)
    fi
done

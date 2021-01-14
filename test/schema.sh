#!/bin/bash

# audo dump lightning compatible schema file

HOST=$1
PORT=$2
DATABASES=$3

mysqldump --defaults-extra-file=my.cnf \
    -h "${HOST:-127.0.0.1}" -P "${PORT:-3306}" \
    --skip-triggers \
    --skip-lock-tables \
    --compact \
    --no-data \
    --set-gtid-purged=OFF \
    --databases ${DATABASES} | \
grep -v '^CREATE DATABASE\|^DROP TABLE \|50001 DROP VIEW IF EXISTS\|^SET \|50001 SET @\|50001 SET c\|^--\|^/\*!4' > schema.sql

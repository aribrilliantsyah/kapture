#!/bin/bash
# Generate fake CRI log files for local testing
set -e

DIR="${1:-testdata/containers}"
mkdir -p "$DIR"

NOW=$(date -u +%Y-%m-%dT%H:%M:%S)
DATE=$(date -u +%Y-%m-%d)

# Simulated container IDs (64 hex chars)
CID1="a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"
CID2="b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3"
CID3="c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4"
CID4="d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5"
CID5="e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6"

# api-server deployment (3 replicas)
for i in 1 2 3; do
  FILE="$DIR/api-server-7f8b9c6d4-x${i}k${i}p_production_app-${CID1:0:$((60+i))}${CID1:$((60+i))}.log"
  echo "Generating $FILE"
  for j in $(seq 1 50); do
    TS="${DATE}T$(printf '%02d:%02d:%02d' $((j/3600)) $(((j%3600)/60)) $((j%60))).${j}00000000Z"
    LEVEL="stdout"
    MSG="INFO  Request handled successfully path=/api/v${j} status=200 duration=${j}ms"
    if [ $((j % 10)) -eq 0 ]; then
      MSG="ERROR Connection timeout after 30s host=db-master.database.svc"
      LEVEL="stderr"
    elif [ $((j % 7)) -eq 0 ]; then
      MSG="WARN  Slow query detected duration=${j}00ms query=\"SELECT * FROM users\""
    fi
    echo "${TS} ${LEVEL} F ${MSG}" >> "$FILE"
  done
done

# postgres statefulset
for i in 0 1 2; do
  FILE="$DIR/postgres-${i}_database_postgres-${CID2:0:64}.log"
  echo "Generating $FILE"
  for j in $(seq 1 30); do
    TS="${DATE}T$(printf '%02d:%02d:%02d' $((j/3600)) $(((j%3600)/60)) $((j%60))).${j}00000000Z"
    echo "${TS} stdout F INFO  PostgreSQL checkpoint complete lsn=0/${j}000000" >> "$FILE"
  done
done

# redis standalone
FILE="$DIR/redis-cache-0_cache_redis-${CID3:0:64}.log"
echo "Generating $FILE"
for j in $(seq 1 20); do
  TS="${DATE}T$(printf '%02d:%02d:%02d' $((j/3600)) $(((j%3600)/60)) $((j%60))).${j}00000000Z"
  echo "${TS} stdout F INFO  DB saved on disk keys=${j}000 rdb_changes=100" >> "$FILE"
done

# worker deployment (2 replicas)
for i in 1 2; do
  FILE="$DIR/worker-batch-5a6b7c-r${i}s${i}t_production_worker-${CID4:0:64}.log"
  echo "Generating $FILE"
  for j in $(seq 1 40); do
    TS="${DATE}T$(printf '%02d:%02d:%02d' $((j/3600)) $(((j%3600)/60)) $((j%60))).${j}00000000Z"
    echo "${TS} stdout F INFO  Job processed job_id=job-${j} duration=${j}0ms" >> "$FILE"
  done
done

# cronjob
FILE="$DIR/backup-28456123-abc12_operations_backup-${CID5:0:64}.log"
echo "Generating $FILE"
for j in $(seq 1 10); do
  TS="${DATE}T$(printf '%02d:%02d:%02d' $((j/3600)) $(((j%3600)/60)) $((j%60))).${j}00000000Z"
  echo "${TS} stdout F INFO  Backup progress: ${j}0% complete" >> "$FILE"
done

echo ""
echo "Generated test log files in $DIR:"
ls -la "$DIR/"
echo ""
echo "Run agent:      make run-agent"
echo "Run aggregator: make run-aggregator"
echo "Dashboard:      http://localhost:19488"

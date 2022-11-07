#!/bin/sh

VARS=$(mktemp)
export VARS

for CLUSTER in ${CLUSTERS}; do
  if [ "${CLUSTER}" = "prod-fss" ] || [ "${CLUSTER}" = "prod-gcp" ]; then
    POOL=nav-prod
  elif [ "${CLUSTER}" = "dev-fss" ] || [ "${CLUSTER}" = "dev-gcp" ]; then
    POOL=nav-dev
  elif [ -n "${TENANT}" ]; then
    POOL="${TENANT}-${CLUSTER}"
  fi

  export CLUSTER
  {
    echo "---"
    echo "now: $(date +%s)000000000"
    echo "image: ${IMAGE}"
    echo "groupid: ${CLUSTER}"
    echo "pool: ${POOL}"
    echo "team: ${TEAM:=aura}"
    echo "canary_kafka_topic: ${TEAM}.${TOPIC_BASE:-kafkarator-canary}-${CLUSTER}"
  } > "${VARS}"
  cat "${VARS}"
  echo "Deploying to ${CLUSTER}..."
  /app/deploy --wait=false
done

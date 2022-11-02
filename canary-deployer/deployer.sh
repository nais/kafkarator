#!/bin/sh

VARS=$(mktemp)
export VARS

for CLUSTER in ${CLUSTERS}; do
  if [ "${CLUSTER}" = "prod-fss" ] || [ "${CLUSTER}" = "prod-gcp" ]; then
    POOL=nav-prod
  elif [ "${CLUSTER}" = "dev-fss" ] || [ "${CLUSTER}" = "dev-gcp" ]; then
    POOL=nav-dev
  else
    POOL="${TENANT:-nav}-${CLUSTER}"
  fi

  export CLUSTER
  {
    echo "---"
    echo "now: $(date +%s)000000000"
    echo "image: ${IMAGE}"
    echo "groupid: ${CLUSTER}"
    echo "pool: ${POOL}"
    echo "team: ${TEAM:-aura}"
    echo "canary_kafka_topic: ${TOPIC:-aura.kafkarator-canary-${CLUSTER}}"
  } > "${VARS}"
  cat "${VARS}"
  echo "Deploying to ${CLUSTER}..."
  /app/deploy --wait=false
done

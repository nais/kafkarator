#!/bin/sh

VARS=$(mktemp -t vars.XXXXXX)
export VARS
RESOURCE=$(mktemp -t resources.XXXXXX)
export RESOURCE

if [ -z "${CLUSTER_POOLS}" ] && [ -n "${CLUSTERS}" ]; then
  for CLUSTER in ${CLUSTERS}; do
    if [ "${CLUSTER}" = "prod-fss" ] || [ "${CLUSTER}" = "prod-gcp" ]; then
      CLUSTER_POOLS="${CLUSTER_POOLS} ${CLUSTER}=nav-prod"
    elif [ "${CLUSTER}" = "dev-fss" ] || [ "${CLUSTER}" = "dev-gcp" ]; then
      CLUSTER_POOLS="${CLUSTER_POOLS} ${CLUSTER}=nav-dev"
    fi
  done
fi

for CLUSTER_POOL in ${CLUSTER_POOLS}; do
  CLUSTER=${CLUSTER_POOL%%=*}
  POOL=${CLUSTER_POOL##*=}

  export CLUSTER
  {
    echo "---"
    echo "now: $(date +%s)000000000"
    echo "image: ${IMAGE}"
    echo "groupid: ${CLUSTER}"
    echo "pool: ${POOL}"
    echo "team: ${TEAM:=aura}"
    echo "canary_kafka_topic: ${TEAM}.${TOPIC_BASE:-kafka-canary}-${CLUSTER}"
    echo "topic_name: ${TOPIC_BASE:-kafka-canary}-${CLUSTER}"
    echo "cluster_name: ${CLUSTER}"
    echo "tenant: ${TENANT:-nav}"
    echo "alert_enabled: ${ALERT_ENABLED:-false}"
  } > "${VARS}"
  cat "${VARS}"
  cat /canary/*.yaml > "${RESOURCE}"
  echo "Deploying to ${CLUSTER}..."
  /app/deploy --wait=false || exit 1
done

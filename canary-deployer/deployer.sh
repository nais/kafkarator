#!/bin/sh

VARS=$(mktemp)
export VARS

# TODO: TEAM
# TODO: TOPIC

for CLUSTER in $CLUSTERS; do
  if [ -z "${POOL}" ]; then
    if [ "$CLUSTER" = "prod-fss" -o "$CLUSTER" = "prod-gcp" ]; then
      POOL=nav-prod
    elif [ "$CLUSTER" = "dev-fss" -o "$CLUSTER" = "dev-gcp" ]; then
      POOL=nav-dev
    fi
  fi
  export CLUSTER
  {
    echo "---"
    echo "image: $IMAGE"
    echo "now: $(date +%s)000000000"
    echo "groupid: $CLUSTER"
    echo "pool: $POOL"
    echo "team: $TEAM"
    echo "canary_kafka_topic: $TOPIC"
  } > "$VARS"
  cat "$VARS"
  echo "Deploying to $CLUSTER..."
  /app/deploy --wait=false
done

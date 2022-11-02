#!/bin/sh

VARS=$(mktemp)
export VARS

for CLUSTER in $CLUSTERS; do
  if [ "$CLUSTER" = "prod-fss" -o "$CLUSTER" = "prod-gcp" -o "$CLUSTER" = "prod-sbs" ]; then
    POOL=nav-prod
  else
    POOL=nav-dev
  fi
  export CLUSTER
  {
    echo "---"
    echo "image: $IMAGE"
    echo "now: $(date +%s)000000000"
    echo "groupid: $CLUSTER"
    echo "pool: $POOL"
  } > "$VARS"
  cat "$VARS"
  echo "Deploying to $CLUSTER..."
  /app/deploy --wait=false
done

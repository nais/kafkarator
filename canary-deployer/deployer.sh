#!/bin/sh

VARS=$(mktemp)
export VARS

for CLUSTER in $CLUSTERS; do
  if [ "dev-gcp" = "$CLUSTER" -o "dev-fss" = "$CLUSTER" -o "dev-sbs" = "$CLUSTER" ]; then
    POOL=nav-dev
  else
    POOL=nav-prod
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

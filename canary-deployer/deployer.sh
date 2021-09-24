#!/bin/sh

export VARS=`mktemp`

VERSION=1.2.3
POOL=nav-prod
for CLUSTER in $CLUSTERS; do
  if "dev-gcp" == $CLUSTER; then
      POOL=nav-dev
  fi
  export CLUSTER
  echo "---" > $VARS
  echo "version: $VERSION"
  echo "now: $(date +%s)000000000" >> $VARS
  echo "groupid: $CLUSTER" >> $VARS
  echo "pool: $POOL" >> $VARS
  echo "Deploying to $CLUSTER..."
  /app/deploy --wait=false
done

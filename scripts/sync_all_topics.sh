#!/usr/bin/bash

# Forces sync of all topics in all namespaces by removing sync hash from status

#kubectl get ns -ocustom-columns=NAME:.metadata.name --no-headers > namespaces

while IFS="" read -r ns; do

  echo "Resyncing topics in ${ns}"
  for topic in $(kubectl get topic "--namespace=${ns}" -ocustom-columns=NAME:.metadata.name --no-headers); do
    kubectl patch topic "--namespace=${ns}" "${topic}" --type=merge --patch '{"status":{"synchronizationHash":"reset"}}'
    sleep 1
  done

  echo "Resyncing streams in ${ns}"
  for stream in $(kubectl get stream "--namespace=${ns}" -ocustom-columns=NAME:.metadata.name --no-headers); do
    kubectl patch stream "--namespace=${ns}" "${stream}" --type=merge --patch '{"status":{"synchronizationHash":"reset"}}'
    sleep 1
  done

done < namespaces

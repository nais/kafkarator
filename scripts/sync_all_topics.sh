#!/usr/bin/bash

# Forces sync of all topics in all namespaces by removing sync hash from status

for ns in $(kubectl get ns -ocustom-columns=NAME:.metadata.name --no-headers); do
  echo "Resyncing topics in ${ns}"
  for topic in $(kubectl get topic "--namespace=${ns}" -ocustom-columns=NAME:.metadata.name --no-headers); do
    kubectl patch topic "--namespace=${ns}" "${topic}" --type=merge --patch '{"status":{"synchronizationHash":"reset"}}'

  done

done

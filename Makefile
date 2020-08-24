# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

kafkarator:
	go build -o bin/kafkarator cmd/kafkarator/*.go

manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
    @{ \
    set -e ;\
    CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
    cd $$CONTROLLER_GEN_TMP_DIR ;\
    go mod init tmp ;\
    go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.5 ;\
    rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
    }
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

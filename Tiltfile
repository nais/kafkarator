load('ext://helm_resource', 'helm_resource', 'helm_repo')
load('ext://local_output', 'local_output')

APP_NAME="kafkarator"

helm_resource('nais-crds', '../liberator/charts', pod_readiness="ignore")

ignore = str(read_file(".earthignore")).split("\n")
custom_build(
    ref=APP_NAME,
    command="earthly +docker-kafkarator --VERSION=$EXPECTED_TAG --REGISTRY=$EXPECTED_REGISTRY",
    deps=["cmd", "controllers", "pkg", "go.mod", "go.sum", "Earthfile"],
    skips_local_docker=False,
    ignore=ignore,
)

# Deployed to the cluster
k8s_yaml(helm("charts/{}".format(APP_NAME), set=[
    # Make sure the chart refers to the same image ref as the one we built
    "image.repository={}".format(APP_NAME),
    # Application configure for testing
    "aiven.projects=dev-nais-dev",
    "aiven.token=" + os.getenv('AIVEN_TOKEN'),
    # Debugger settings
    "debugger.deployment=false",
    "debugger.service=false",
]))
kafkarator_objects = [
    "chart-kafkarator:NetworkPolicy",
    "chart-kafkarator:Secret",
    "chart-kafkarator:ServiceAccount",
    "chart-kafkarator:ClusterRole",
    "chart-kafkarator:ClusterRoleBinding",
]
k8s_resource(
    workload="chart-{}".format(APP_NAME),
    resource_deps=["nais-crds"],
    objects=kafkarator_objects,
)

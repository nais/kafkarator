#!/usr/bin/env python3
import argparse
import json
import subprocess
from dataclasses import dataclass

from common import AivenKafka, User


@dataclass(frozen=True)
class Secret:
    username: str
    name: str
    namespace: str
    context: str


def get_secrets_in(context, team):
    cmd = [
        "kubectl",
        "get", "secret",
        "--context", context,
        "--output", "json",
        "--selector", "type=aivenator.aiven.nais.io"
    ]
    if team:
        cmd.extend(("--namespace", team))
    else:
        cmd.append("--all-namespaces")
    print(f"Executing {' '.join(cmd)}")
    output = subprocess.check_output(cmd)
    data = json.loads(output)
    for item in data["items"]:
        metadata = item["metadata"]
        name = metadata["name"]
        namespace = metadata["namespace"]
        annotations = metadata.get("annotations", {})
        username = annotations.get("kafka.aiven.nais.io/serviceUser")
        if username:
            yield Secret(username, name, namespace, context)


def get_secrets(contexts, team) -> set[Secret]:
    secrets = set()
    for context in contexts:
        secrets.update(set(get_secrets_in(context, team)))
    return secrets


def find_unused_users(secrets: set[Secret], users: set[User]) -> set[str]:
    secret_usernamess = {s.username for s in secrets}
    user_usernames = {u.username for u in users}
    return user_usernames - secret_usernamess


def main(env, dry_run, team):
    project = f"nav-{env}"
    contexts = {f"{env}-{kind}" for kind in ("fss", "gcp")}

    aiven = AivenKafka(project, dry_run=dry_run)

    service = aiven.get_service(team)
    print(f"Aiven knows {len(service.users)} users")
    secrets = get_secrets(contexts, team)
    print(f"Found {len(secrets)} secrets in all clusters")
    users_to_delete = set(find_unused_users(secrets, service.users))
    print(f"Found {len(users_to_delete)} users with no associated secret")
    aiven.delete_users(users_to_delete)


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument("-n", "--dry-run", action="store_true", help="Make no actual changes")
    parser.add_argument("-t", "--team", action="store", help="Only operate on users/secrets belonging to team")
    parser.add_argument("env", action="store", help="Environment to process")
    options = parser.parse_args()
    main(options.env, options.dry_run, options.team)

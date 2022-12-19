#!/usr/bin/env python3
import argparse
import json
import os
import re
import subprocess
from dataclasses import dataclass

import requests

OPERATOR_PATTERNS = (
    re.compile(r"[^_]+_[^_]+_[^_]+_.+"),
    re.compile(r".*\..*"),
)


class AivenAuth(requests.auth.AuthBase):
    def __init__(self, token=None):
        if token is None:
            token = os.getenv("AIVEN_TOKEN")
        self.token = token

    def __call__(self, r):
        r.headers["authorization"] = f"Bearer {self.token}"
        return r


@dataclass(frozen=True)
class Secret:
    username: str
    name: str
    namespace: str
    context: str


@dataclass(frozen=True)
class User:
    username: str
    type: str


@dataclass
class Service:
    users: set[User]


class AivenKafka(object):
    base = "https://api.aiven.io/v1/project"

    def __init__(self, project, service=None, dry_run=False):
        self.project = project
        if service is None:
            service = project + "-kafka"
        self.service = service
        self.dry_run = dry_run
        self.session = requests.Session()
        self.session.auth = AivenAuth()
        self.base_url = f"{self.base}/{self.project}/service/{self.service}"

    def get_service(self, team):
        resp = self.session.get(self.base_url)
        resp.raise_for_status()
        data = resp.json()
        return Service(set(self.extract_users(data, team)))

    @staticmethod
    def extract_users(data, team):
        for u in data["service"]["users"]:
            if any(p.match(u["username"]) for p in OPERATOR_PATTERNS):
                if not team or u["username"].startswith(team):
                    yield User(u["username"], u["type"])
            else:
                print(f"Ignoring {u['username']}, since it did not match any pattern")

    def delete_users(self, users_to_delete: set[str]):
        for username in users_to_delete:
            if not self.dry_run:
                print(f"Deleting {username}")
                resp = self.session.delete(f"{self.base_url}/user/{username}")
                resp.raise_for_status()
            else:
                print(f"Would have deleted {username}")


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

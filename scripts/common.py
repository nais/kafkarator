import json
import os
import re
import subprocess
from dataclasses import dataclass, field
from typing import Optional

import requests
from alive_progress import alive_it

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
class Acl:
    id: str
    permission: str
    topic: str
    username: str


@dataclass(frozen=True)
class User:
    username: str
    type: str


@dataclass
class Service:
    acls: set[Acl]
    topics: set[str]
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

    def get_topics(self):
        resp = self.session.get(f"{self.base_url}/topic")
        resp.raise_for_status()
        data = resp.json()
        return [t["topic_name"] for t in data["topics"]]

    def get_service(self, team=None):
        resp = self.session.get(self.base_url)
        resp.raise_for_status()
        data = resp.json()
        acls = {Acl(**a) for a in data["service"]["acl"]}
        topics = {t["topic_name"] for t in data["service"]["topics"]}
        users = set(self.extract_users(data, team))
        return Service(acls, topics, users)

    def get_acls(self):
        resp = self.session.get(f"{self.base_url}/acl")
        resp.raise_for_status()
        data = resp.json()
        return {Acl(**a) for a in data["acl"]}

    def delete_acls(self, acls_to_delete):
        for acl in alive_it(acls_to_delete, title="Deleting ACLs"):
            if not self.dry_run:
                print(f"Deleting {acl}")
                resp = self.session.delete(f"{self.base_url}/acl/{acl.id}")
                resp.raise_for_status()
            else:
                print(f"Would have deleted {acl}")

    def delete_users(self, users_to_delete):
        for username in alive_it(users_to_delete, title="Deleting users"):
            if not self.dry_run:
                print(f"Deleting {username}")
                resp = self.session.delete(f"{self.base_url}/user/{username}")
                resp.raise_for_status()
            else:
                print(f"Would have deleted {username}")

    @staticmethod
    def extract_users(data, team):
        for u in data["service"]["users"]:
            if any(p.match(u["username"]) for p in OPERATOR_PATTERNS):
                if not team or u["username"].startswith(team):
                    yield User(u["username"], u["type"])
            else:
                print(f"Ignoring {u['username']}, since it did not match any pattern")


@dataclass(frozen=True)
class Secret:
    username: str
    name: str
    namespace: str
    context: str
    data: Optional[dict] = field(default_factory=dict, repr=False, compare=False)


def get_secrets_in(context, team=None, project=None):
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
        pool = annotations.get("kafka.aiven.nais.io/pool")
        if username and (project is None or project == pool):
            yield Secret(username, name, namespace, context, item)

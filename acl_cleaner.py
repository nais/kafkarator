#!/usr/bin/env python
import os
from dataclasses import dataclass
from fnmatch import fnmatch

import requests


class AivenAuth(requests.auth.AuthBase):
    def __init__(self, token=None):
        if token is None:
            token = os.getenv("AIVEN_TOKEN")
        self.token = token

    def __call__(self, r):
        r.headers["authorization"] = f"Bearer {self.token}"
        return r


@dataclass
class Acl:
    id: str
    permission: str
    topic: str
    username: str


@dataclass
class Service:
    acls: list[Acl]
    topics: set[str]
    users: set[str]


class AivenKafka(object):
    base = "https://api.aiven.io/v1/project"

    def __init__(self, project, service=None):
        self.project = project
        if service is None:
            service = project + "-kafka"
        self.service = service
        self.session = requests.Session()
        self.session.auth = AivenAuth()
        self.base_url = f"{self.base}/{self.project}/service/{self.service}"

    def get_topics(self):
        resp = self.session.get(f"{self.base_url}/topic")
        resp.raise_for_status()
        data = resp.json()
        return [t["topic_name"] for t in data["topics"]]

    def get_service(self):
        resp = self.session.get(self.base_url)
        resp.raise_for_status()
        data = resp.json()
        acls = [Acl(**a) for a in data["service"]["acl"]]
        topics = {t["topic_name"] for t in data["service"]["topics"]}
        users = {u["username"] for u in data["service"]["users"]}
        return Service(acls, topics, users)

    def get_acls(self):
        resp = self.session.get(f"{self.base_url}/acl")
        resp.raise_for_status()
        data = resp.json()
        return [Acl(**a) for a in data["acl"]]

    def delete_acls(self, acls_to_delete):
        for acl in acls_to_delete:
            print(f"Deleting {acl}")
            resp = self.session.delete(f"{self.base_url}/acl/{acl.id}")
            resp.raise_for_status()

    def delete_users(self, users_to_delete):
        for username in users_to_delete:
            print(f"Deleting {username}")
            resp = self.session.delete(f"{self.base_url}/user/{username}")
            resp.raise_for_status()


def find_unused_acls(topics, acls):
    for acl in acls:
        if acl.topic.startswith("__") or acl.topic in topics:
            continue
        yield acl


def find_unused_users(acls, users):
    acl_patterns = {acl.username for acl in acls}
    for username in users:
        if username == "avnadmin":
            continue
        if not any(fnmatch(username, pattern) for pattern in acl_patterns):
            yield username


def main():
    aiven = AivenKafka("nav-prod")

    service = aiven.get_service()

    acls_to_delete = list(find_unused_acls(service.topics, service.acls))
    print(f"Found {len(acls_to_delete)} ACLs referencing non-existing topics")
    aiven.delete_acls(acls_to_delete)

    acls = aiven.get_acls()
    users_to_delete = list(find_unused_users(acls, service.users))
    print(f"Found {len(users_to_delete)} users with no associated ACLs")
    aiven.delete_users(users_to_delete)


if __name__ == '__main__':
    main()

#!/usr/bin/env python
import argparse
import os
import tempfile

import pyaml
import requests
from pydantic import BaseModel

REQUIREMENTS = (
    "requests",
    "requests-toolbelt",
    "pyaml",
    "pydantic",
)

TOPIC_NAME_FORMAT = "leesah-quiz-{event}-{i}"

TOPIC_CONFIG = {
    "cleanup_policy": "delete",  # delete, compact, compact,delete
    "min_insync_replicas": 3,
    "retention_bytes": -1,  # -1 means unlimited
    "retention_ms": 24 * 60 * 60 * 1000,  # 48 hours
}

USER_NAME = "leesah-quiz-master"
ACCESS_LEVEL = "admin"


class AivenAuth(requests.auth.AuthBase):
    def __init__(self, token=None):
        if token is None:
            token = os.getenv("AIVEN_TOKEN")
        self.token = token

    def __call__(self, r):
        r.headers["authorization"] = f"Bearer {self.token}"
        return r


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

    def get_service(self):
        resp = self.session.get(self.base_url)
        resp.raise_for_status()
        data = resp.json()
        return Service.model_validate(data["service"])

    def get_ca(self):
        url = f"{self.base}/{self.project}/kms/ca"
        resp = self.session.get(url)
        resp.raise_for_status()
        data = resp.json()
        return data["certificate"]

    def create_topic(self, service, topic_name):
        try:
            return service.find_topic(topic_name)
        except RuntimeError:
            pass
        body = {
            "config": TOPIC_CONFIG,
            "partitions": 1,
            "replication": 3,
            "topic_name": topic_name,
        }
        url = f"{self.base_url}/topic"
        resp = self.session.post(url, json=body)
        resp.raise_for_status()

    def create_acl(self, service, topic_name, user_name, access_level):
        try:
            return service.find_acl(topic_name, user_name, access_level)
        except RuntimeError:
            pass
        body = {
            "permission": access_level,
            "topic": topic_name,
            "username": user_name,
        }
        url = f"{self.base_url}/acl"
        resp = self.session.post(url, json=body)
        resp.raise_for_status()

    def create_user(self, service, user_name):
        try:
            return service.find_user(user_name)
        except RuntimeError:
            pass
        body = {
            "username": user_name,
        }
        url = f"{self.base_url}/user"
        resp = self.session.post(url, json=body)
        resp.raise_for_status()
        data = resp.json()
        return User.parse_obj(data["user"])


class Acl(BaseModel):
    permission: str
    topic: str
    username: str


class Component(BaseModel):
    component: str
    host: str
    port: int


class Topic(BaseModel):
    cleanup_policy: str
    min_insync_replicas: int
    partitions: int
    replication: int
    retention_bytes: int
    retention_hours: int
    topic_name: str


class User(BaseModel):
    access_cert: str
    access_key: str
    username: str


class Service(BaseModel):
    acl: list[Acl]
    components: list[Component]
    topics: list[Topic]
    users: list[User]

    def find_user(self, user_name):
        for user in self.users:
            if user.username == user_name:
                return user
        raise RuntimeError(f"user {user_name} not found")

    def find_topic(self, topic_name):
        for topic in self.topics:
            if topic.topic_name == topic_name:
                return topic
        raise RuntimeError(f"topic {topic_name} not found")

    def find_acl(self, topic_name, user_name, access_level):
        for acl in self.acl:
            if acl.permission == access_level and acl.username == user_name and acl.topic == topic_name:
                return acl
        raise RuntimeError(f"acl for {topic_name}, {user_name} and {access_level} not found")

    def get_broker(self):
        for component in self.components:
            if component.component == "kafka":
                return f"{component.host}:{component.port}"
        raise RuntimeError("kafka component not found")


class Packet(BaseModel):
    user: User
    ca: str
    broker: str
    topics: list[str]


def main(event, count):
    import logging
    logging.basicConfig(level=logging.DEBUG)
    kafka = AivenKafka("nav-integration-test")
    service = kafka.get_service()
    actual_topics = [TOPIC_NAME_FORMAT.format(event=event, i=i) for i in range(1, count+1)]
    for topic_name in actual_topics:
        kafka.create_topic(service, topic_name)
        kafka.create_acl(service, topic_name, USER_NAME, ACCESS_LEVEL)
    packet = Packet(
        user=kafka.create_user(service, USER_NAME),
        ca=kafka.get_ca(),
        broker=service.get_broker(),
        topics=actual_topics,
    )
    with tempfile.NamedTemporaryFile(prefix="leesah-quiz-master", suffix=".yaml", delete=False) as fobj:
        pyaml.dump(packet.model_dump(), fobj)
        print(f"Packet saved to {fobj.name}")


if __name__ == '__main__':
    try:
        parser = argparse.ArgumentParser()
        parser.add_argument("event", help="Name of event")
        parser.add_argument("count", nargs="?", type=int, default=1, help="Number of topics to create")
        options = parser.parse_args()
        main(options.event, options.count)
    except requests.exceptions.RequestException as re:
        from requests_toolbelt.utils import dump

        data = dump.dump_all(re.response)
        print(data.decode("utf-8"))
        raise

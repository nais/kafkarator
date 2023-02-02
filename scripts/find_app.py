#!/usr/bin/env python3


import argparse
import base64
import textwrap

from alive_progress import alive_it

from common import get_secrets_in


def get_secrets_for_user(env, username):
    contexts = {f"{env}-{kind}" for kind in ("fss", "gcp")}
    for context in contexts:
        for secret in alive_it(get_secrets_in(context)):
            if secret.username == username:
                yield secret


def extract_updated_at(secrets):
    encoded_value = secrets.get("AIVEN_SECRET_UPDATED", secrets.get("KAFKA_SECRET_UPDATED"))
    return base64.b64decode(encoded_value).decode("utf-8")


def print_secret_info(secret):
    data = secret.data.get("data", {})
    metadata = secret.data.get("metadata", {})
    labels = metadata.get("labels", {})
    annotations = metadata.get("annotations", {})
    print(textwrap.dedent(f"""\
        secretname: {secret.name}
        username: {secret.username}
        context: {secret.context}
        updated_at: {extract_updated_at(data)}
        secret_created_at: {metadata.get("creationTimestamp")}
        team: {labels.get("team")}
        namespace: {secret.namespace}
        app: {labels.get("app")}
        protected: {annotations.get("aivenator.aiven.nais.io/protected")}
    """))


def main(env, username):
    secrets = list(get_secrets_for_user(env, username))
    print(f"Found {len(secrets)} secrets for {username}")
    for secret in secrets:
        print_secret_info(secret)


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument("env", action="store", help="Environment to process")
    parser.add_argument("username", action="store", help="Username to search for")
    options = parser.parse_args()
    main(options.env, options.username)

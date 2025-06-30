#!/usr/bin/env -S uv run

# To run this application, use:
#   uv run find_app.py
#
# /// script
# requires-python = ">=3.12"
# dependencies = [
#   "rich",
#   "requests",
# ]
# ///


import argparse
import base64
import re

from rich import print
from rich.table import Table

from common import get_secrets_in


def get_secrets_for_user(env, username):
    pattern = re.compile(username)
    contexts = {f"nav-{env}-{kind}" for kind in ("fss", "gcp")}
    for context in contexts:
        for secret in get_secrets_in(context):
            if pattern.match(secret.username):
                yield secret


def extract_updated_at(secrets):
    encoded_value = secrets.get("AIVEN_SECRET_UPDATED", secrets.get("KAFKA_SECRET_UPDATED"))
    return base64.b64decode(encoded_value).decode("utf-8")


def print_secret_info(secret):
    data = secret.data.get("data", {})
    metadata = secret.data.get("metadata", {})
    labels = metadata.get("labels", {})
    annotations = metadata.get("annotations", {})
    table = Table(show_header=False)
    table.add_column("Key")
    table.add_column("Value")
    table.add_row("secretname", secret.name)
    table.add_row("username", secret.username)
    table.add_row("context", secret.context)
    table.add_row("updated_at", extract_updated_at(data))
    table.add_row("secret_created_at", metadata.get("creationTimestamp"))
    table.add_row("team", labels.get("team"))
    table.add_row("namespace", secret.namespace)
    table.add_row("app", labels.get("app"))
    table.add_row("protected", annotations.get("aivenator.aiven.nais.io/protected"))
    print(table)


def main(env, username):
    secrets = list(get_secrets_for_user(env, username))
    print(f"Found {len(secrets)} secrets for {username}")
    for secret in secrets:
        print_secret_info(secret)


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument("env", action="store", help="Environment to process")
    parser.add_argument("username", action="store", help="Username to search for (can be regex)")
    options = parser.parse_args()
    main(options.env, options.username)

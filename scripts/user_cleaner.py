#!/usr/bin/env python3
import argparse

from rich import print

from common import AivenKafka, User, Secret, get_secrets_in


def get_secrets(contexts, team, project) -> set[Secret]:
    secrets = set()
    for context in contexts:
        secrets.update(set(get_secrets_in(context, team, project)))
    return secrets


def find_unused_users(secrets: set[Secret], users: set[User]) -> set[str]:
    secret_usernamess = {s.username for s in secrets}
    user_usernames = {u.username for u in users}
    return user_usernames - secret_usernamess


def main(env, dry_run, team):
    project = f"nav-{env}"
    if env != "infrastructure":
        contexts = {f"nav-{env}-{kind}" for kind in ("fss", "gcp")}
    else:
        contexts = {f"nav-{e}-{kind}" for kind in ("fss", "gcp") for e in ("dev", "prod")}

    aiven = AivenKafka(project, dry_run=dry_run)

    service = aiven.get_service(team)
    print(f"Aiven knows {len(service.users)} users")
    secrets = get_secrets(contexts, team, project)
    print(f"Found {len(secrets)} secrets in all clusters")
    users_to_delete = set(find_unused_users(secrets, service.users))
    print(f"Found {len(users_to_delete)} users with no associated secret")
    aiven.delete_users(users_to_delete)


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument("-n", "--dry-run", action="store_true", help="Make no actual changes")
    parser.add_argument("-t", "--team", action="store", help="Only operate on users/secrets belonging to team")
    parser.add_argument("env", choices=("dev", "prod", "infrastructure"), action="store", help="Environment to process")
    options = parser.parse_args()
    main(options.env, options.dry_run, options.team)

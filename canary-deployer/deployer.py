#!/usr/bin/env python3
import json
import logging
import sys
import tempfile
from datetime import datetime
from enum import Enum
from subprocess import CalledProcessError
from typing import List, Optional

import trio
from pydantic import BaseSettings, BaseModel


class DeployConfig(BaseModel):
    pool: str
    canary_cluster: str
    topic_cluster: Optional[str] = None


LogLevel = Enum("LogLevel", {k: k for k in logging._nameToLevel.keys()})  # NOQA


class Settings(BaseSettings):
    image: str
    team: str = "nais-verification"
    deploy_configs: List[DeployConfig]
    log_level: LogLevel = LogLevel[logging.getLevelName(logging.INFO)]
    dry_run: bool = False


async def count_to_ten():
    for i in range(10):
        await trio.sleep(1)
        print(f"Pretending to wait for deploy to complete ... step {i}")


async def _execute_deploy(cluster, resource_name, vars_file_name, settings, logger):
    cmd = [
        "/app/deploy",
        "--cluster", cluster,
        "--resource", resource_name,
        "--vars", vars_file_name,
        "--team", settings.team,
        "--owner", "nais",
        "--repository", "nais/kafkarator",
    ]
    if settings.dry_run:
        logger.info("Would have executed command: %s", " ".join(cmd))
        await count_to_ten()
    else:
        await trio.run_process(cmd)


async def deploy_canary(config: DeployConfig, settings: Settings):
    logger = logging.getLogger(f"deploy-canary-{config.canary_cluster}")
    logger.info("Deploying canary to %s", config.canary_cluster)
    with tempfile.NamedTemporaryFile("w", prefix=f"canary-vars-{config.canary_cluster}", suffix=".yaml") as vars_file:
        data = {
            "team": settings.team,
            "image": settings.image,
            "pool": config.pool,
            "now": datetime.now().isoformat(),
            "canary_kafka_topic": f"{settings.team}.kafka-canary-{config.canary_cluster}",
            "groupid": config.canary_cluster,
        }
        json.dump(data, vars_file)
        vars_file.flush()
        logger.debug(json.dumps(data, indent=2))
        try:
            await _execute_deploy(config.canary_cluster, "/canary/canary.yaml", vars_file.name, settings, logger)
        except CalledProcessError:
            raise RuntimeError(f"Error when deploying canary to {config.canary_cluster}") from None
    logger.info("Completed deploying canary to %s", config.canary_cluster)


async def deploy_topic(config: DeployConfig, settings: Settings):
    topic_name = f"kafka-canary-{config.canary_cluster}"
    logger = logging.getLogger(f"deploy-topic-{config.canary_cluster}")
    logger.info("Deploying topic %s to %s", topic_name, config.topic_cluster)
    with tempfile.NamedTemporaryFile("w", prefix=f"topic-vars-{config.canary_cluster}", suffix=".yaml") as vars_file:
        data = {
            "team": settings.team,
            "pool": config.pool,
            "topic_name": topic_name,
        }
        json.dump(data, vars_file)
        vars_file.flush()
        logger.debug(json.dumps(data, indent=2))
        try:
            await _execute_deploy(config.topic_cluster, "/canary/topic.yaml", vars_file.name, settings, logger)
        except CalledProcessError:
            raise RuntimeError(f"Error when deploying topic {topic_name} to {config.topic_cluster}") from None
    logger.info("Completed deploying topic %s to %s", topic_name, config.topic_cluster)


def _set_topic_cluster(deploy_config: DeployConfig):
    deploy_config.topic_cluster = deploy_config.canary_cluster
    if deploy_config.canary_cluster.endswith("-fss"):
        # Special NAV case
        deploy_config.topic_cluster = deploy_config.canary_cluster.replace("-fss", "-gcp")


async def main(settings: Settings):
    logging.basicConfig(format="[%(asctime)s|%(levelname)5.5s|%(name)-25s] %(message)s", level=settings.log_level.value)
    logging.info("Starting deploy")
    try:
        async with trio.open_nursery() as nursery:
            for deploy_config in settings.deploy_configs:
                _set_topic_cluster(deploy_config)
                nursery.start_soon(deploy_topic, deploy_config, settings)
                nursery.start_soon(deploy_canary, deploy_config, settings)
    except Exception as e:
        logging.exception("Unexpected error: %s", e)
        sys.exit(1)
    logging.info("Deploy complete")


if __name__ == '__main__':
    trio.run(main, Settings())

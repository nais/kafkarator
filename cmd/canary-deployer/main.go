// Command canary-deployer deploys the kafka canary (topics + application) to
// each configured cluster via the Nais API.
//
// Usage:
//
//	canary-deployer provision  # ensure team + service account exist
//	canary-deployer deploy     # mint a token and deploy canary topics + app
package main

import (
	"context"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/nais/kafkarator/internal/canarydeployer"
)

func main() {
	log.SetFormatter(&log.TextFormatter{DisableTimestamp: true})

	if len(os.Args) < 2 {
		log.Error("usage: canary-deployer <provision|deploy>")
		os.Exit(1)
	}

	cfg, err := canarydeployer.ConfigFromEnv()
	if err != nil {
		log.Errorf("%s", err)
		os.Exit(1)
	}

	ctx := context.Background()
	switch os.Args[1] {
	case "provision":
		err = canarydeployer.Provision(ctx, cfg)
	case "deploy":
		err = canarydeployer.Deploy(ctx, cfg)
	default:
		log.Errorf("unknown subcommand %q (want provision|deploy)", os.Args[1])
		os.Exit(1)
	}

	if err != nil {
		log.Errorf("%s", err)
		os.Exit(1)
	}
}

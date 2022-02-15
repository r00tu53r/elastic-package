// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/elastic/elastic-package/internal/builder"
	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/files"
	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/profile"
	"github.com/pkg/errors"
)

func Deploy(options Options) error {
	buildPackagesPath, found, err := builder.FindBuildPackagesDirectory()
	if err != nil {
		return errors.Wrap(err, "finding build packages directory failed")
	}

	stackPackagesDir, err := locations.NewLocationManager()
	if err != nil {
		return errors.Wrap(err, "locating stack packages directory failed")
	}

	err = files.ClearDir(stackPackagesDir.PackagesDir())
	if err != nil {
		return errors.Wrap(err, "clearing package contents failed")
	}

	if found {
		fmt.Printf("Custom build packages directory found: %s\n", buildPackagesPath)
		err = files.CopyAll(buildPackagesPath, stackPackagesDir.PackagesDir())
		if err != nil {
			return errors.Wrap(err, "copying package contents failed")
		}
	}

	fmt.Println("Packages from the following directories will be loaded into the package-registry:")
	fmt.Println("- built-in packages (package-storage:snapshot Docker image)")

	if found {
		fmt.Printf("- %s\n", buildPackagesPath)
	}

	err = dockerComposeBuild(options)
	if err != nil {
		return errors.Wrap(err, "building docker images failed")
	}

	err = stackDeploy(options)
	if err != nil {
		return errors.Wrap(err, "running docker stack deploy failed")
	}
	return nil
}

func stackDeploy(options Options) error {

	var args []string

	composeFile := options.Profile.FetchPath(profile.SnapshotFile)

	args = append(args, "stack")
	args = append(args, "deploy")
	args = append(args, "--compose-file")
	args = append(args, composeFile)
	args = append(args, options.StackName)

	appConfig, err := install.Configuration()
	if err != nil {
		return errors.Wrap(err, "can't read application configuration")
	}

	envs := newEnvBuilder().
		withEnvs(appConfig.StackImageRefs(options.StackVersion).AsEnv()).
		withEnv(stackVariantAsEnv(options.StackVersion)).withEnvs(options.Profile.ComposeEnvVars()).build()

	cmd := exec.Command("docker", args...)
	cmd.Env = append(os.Environ(), envs...)

	if logger.IsDebugMode() {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	logger.Debugf("running command: %s", cmd)
	return cmd.Run()
}

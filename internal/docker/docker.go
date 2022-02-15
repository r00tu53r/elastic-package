// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/logger"
)

// NetworkDescription describes the Docker network and connected Docker containers.
type NetworkDescription struct {
	Containers map[string]struct {
		Name string
	}
}

// ContainerDescription describes the Docker container.
type ContainerDescription struct {
	ID    string
	State struct {
		Status   string
		ExitCode int
		Health   *struct {
			Status string
			Log    []struct {
				Start    time.Time
				ExitCode int
				Output   string
			}
		}
	}
}

// String function dumps string representation of the container description.
func (c *ContainerDescription) String() string {
	b, err := json.Marshal(c)
	if err != nil {
		return "error: can't marshal container description"
	}
	return string(b)
}

// Pull downloads the latest available revision of the image.
func Pull(image string) error {
	cmd := exec.Command("docker", "pull", image)

	if logger.IsDebugMode() {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	logger.Debugf("run command: %s", cmd)
	err := cmd.Run()
	if err != nil {
		return errors.Wrap(err, "running docker command failed")
	}
	return nil
}

// ContainerID function returns the container ID for a given container name.
func ContainerID(containerName string) (string, error) {
	cmd := exec.Command("docker", "ps", "--filter", "name="+containerName, "--format", "{{.ID}}")
	errOutput := new(bytes.Buffer)
	cmd.Stderr = errOutput

	logger.Debugf("output command: %s", cmd)
	output, err := cmd.Output()
	if err != nil {
		return "", errors.Wrapf(err, "could not find \"%s\" container (stderr=%q)", containerName, errOutput.String())
	}
	containerIDs := bytes.Split(bytes.TrimSpace(output), []byte{'\n'})
	if len(containerIDs) != 1 {
		return "", fmt.Errorf("expected single %s container", containerName)
	}
	return string(containerIDs[0]), nil
}

// InspectNetwork function returns the network description for the selected network.
func InspectNetwork(network string) ([]NetworkDescription, error) {
	cmd := exec.Command("docker", "network", "inspect", network)
	errOutput := new(bytes.Buffer)
	cmd.Stderr = errOutput

	logger.Debugf("output command: %s", cmd)
	output, err := cmd.Output()
	if err != nil {
		return nil, errors.Wrapf(err, "could not inspect the network (stderr=%q)", errOutput.String())
	}

	var networkDescriptions []NetworkDescription
	err = json.Unmarshal(output, &networkDescriptions)
	if err != nil {
		return nil, errors.Wrapf(err, "can't unmarshal network inspect for %s (stderr=%q)", network, errOutput.String())
	}
	return networkDescriptions, nil
}

// ConnectToNetwork function connects the container to the selected Docker network.
func ConnectToNetwork(containerID, network string) error {
	cmd := exec.Command("docker", "network", "connect", network, containerID)
	errOutput := new(bytes.Buffer)
	cmd.Stderr = errOutput

	logger.Debugf("run command: %s", cmd)
	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "could not attach container to the stack network (stderr=%q)", errOutput.String())
	}
	return nil
}

func CreateNetwork(name, driver string, arg ...string) error {
	netcmd := []string{
		"network",
		"create",
		"--driver",
		driver,
	}
	if len(arg) > 0 {
		netcmd = append(netcmd, arg...)
	}
	netcmd = append(netcmd, name)
	cmd := exec.Command("docker", netcmd...)
	errOutput := new(bytes.Buffer)
	cmd.Stderr = errOutput
	logger.Debugf("run command: %s", cmd)
	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "could not create stack network (stderr=%q)", errOutput.String())
	}
	return nil
}

// InspectContainers function inspects selected Docker containers.
func InspectContainers(containerIDs ...string) ([]ContainerDescription, error) {
	args := []string{"inspect"}
	args = append(args, containerIDs...)
	cmd := exec.Command("docker", args...)

	errOutput := new(bytes.Buffer)
	cmd.Stderr = errOutput

	logger.Debugf("output command: %s", cmd)
	output, err := cmd.Output()
	if err != nil {
		return nil, errors.Wrapf(err, "could not inspect containers (stderr=%q)", errOutput.String())
	}

	var containerDescriptions []ContainerDescription
	err = json.Unmarshal(output, &containerDescriptions)
	if err != nil {
		return nil, errors.Wrapf(err, "can't unmarshal container inspect for %s (stderr=%q)", strings.Join(containerIDs, ","), errOutput.String())
	}
	return containerDescriptions, nil
}

// Copy function copies resources from the container to the local destination.
func Copy(containerName, containerPath, localPath string) error {
	cmd := exec.Command("docker", "cp", containerName+":"+containerPath, localPath)
	errOutput := new(bytes.Buffer)
	cmd.Stderr = errOutput

	logger.Debugf("run command: %s", cmd)
	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "could not copy files from the container (stderr=%q)", errOutput.String())
	}
	return nil
}

func SwarmInit(ift string) (string, error) {

	swarmArg := []string{
		"swarm",
		"init",
		"--advertise-addr",
		ift,
	}
	cmd := exec.Command("docker", swarmArg...)
	errOutput := new(bytes.Buffer)
	cmd.Stderr = errOutput
	logger.Debugf("run command: %s", cmd)
	if err := cmd.Run(); err != nil {
		return "", errors.Wrapf(err, "docker swarm init failed (stderr=%q)", errOutput.String())
	}
	joinToken, err := swarmJoinToken()
	if err != nil {
		logger.Error(err)
		return "", err
	}
	return joinToken, nil
}

func swarmJoinToken() (string, error) {
	swarmArg := []string{
		"swarm",
		"join-token",
		"worker",
	}
	cmd := exec.Command("docker", swarmArg...)
	errOutput := new(bytes.Buffer)
	cmd.Stderr = errOutput
	logger.Debugf("run command: %s", cmd)
	out, err := cmd.Output()
	if err != nil {
		return "", errors.Wrapf(err, "unable to get join token (stderr=%q)", errOutput.String())
	}
	return string(out), nil
}

func SwarmLeave() error {
	swarmArg := []string{
		"swarm",
		"leave",
		"--force",
	}
	cmd := exec.Command("docker", swarmArg...)
	errOutput := new(bytes.Buffer)
	cmd.Stderr = errOutput
	logger.Debugf("run command: %s", cmd)
	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "docker swarm leave failed (stderr=%q)", errOutput.String())
	}
	return nil
}

func SwarmStackDown(stackName string) error {
	var args []string

	args = append(args, "stack")
	args = append(args, "rm")
	args = append(args, stackName)

	cmd := exec.Command("docker", args...)

	if logger.IsDebugMode() {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	logger.Debugf("running command: %s", cmd)
	return cmd.Run()
}

// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"
	"net"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/docker"
	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/profile"
	"github.com/elastic/elastic-package/internal/stack"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func setupSwarmCommand() *cobra.Command {

	upCommand := &cobra.Command{
		Use:   "up",
		Short: "Start the stack on the docker swarm",
		RunE:  swarmUp,
	}
	upCommand.Flags().StringP(cobraext.StackVersionFlagName, "", install.DefaultStackVersion, cobraext.StackVersionFlagDescription)
	upCommand.Flags().StringP(cobraext.StackNameFlagName, "", "", cobraext.StackNameFlagDescription)

	downCommand := &cobra.Command{
		Use:   "down",
		Short: "Stop the swarm services",
		RunE:  swarmDown,
	}
	downCommand.Flags().StringP(cobraext.StackNameFlagName, "", "", cobraext.StackNameFlagDescription)

	initCommand := &cobra.Command{
		Use:   "init",
		Short: "Create and initialize docker swarm and profile",
		RunE:  stackInit,
	}
	initCommand.Flags().StringP(cobraext.StackOverlayNetworkNameFlagName, "", "elastic-package-stack-overlay",
		cobraext.StackOverlayNetworkNameFlagDescription)
	initCommand.Flags().StringP(cobraext.InterfaceFlagName, "", "", cobraext.InterfaceFlagDescription)
	initCommand.MarkFlagRequired(cobraext.InterfaceFlagName)
	initCommand.Flags().StringP(cobraext.IPSubnetFlagName, "", "", cobraext.IPSubnetFlagDescription)
	initCommand.MarkFlagRequired(cobraext.IPSubnetFlagName)

	leaveCommand := &cobra.Command{
		Use:   "leave",
		Short: "Leave the docker swarm (deletes the swarm and all the associated data)",
		RunE:  swarmLeave,
	}

	swarmCommand := &cobra.Command{
		Use:   "swarm",
		Short: "Setup stack with multi-host networking",
	}
	swarmCommand.AddCommand(initCommand)
	swarmCommand.AddCommand(leaveCommand)
	swarmCommand.AddCommand(upCommand)
	swarmCommand.AddCommand(downCommand)

	return swarmCommand
}

func swarmUp(cmd *cobra.Command, args []string) error {
	cmd.Println("Boot up the Elastic stack (on swarm)")

	stackVersion, err := cmd.Flags().GetString(cobraext.StackVersionFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.StackVersionFlagName)
	}

	profileName, err := cmd.Flags().GetString(cobraext.ProfileFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.ProfileFlagName)
	}

	stackName, err := cmd.Flags().GetString(cobraext.StackNameFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.StackNameFlagName)
	}

	usrProfile, err := profile.LoadProfile(profileName)
	if errors.Is(err, profile.ErrNotAProfile) {
		pList, err := availableProfilesAsAList()
		if err != nil {
			return errors.Wrap(err, "error listing known profiles")
		}
		return fmt.Errorf("%s is not a valid profile, known profiles are: %s", profileName, pList)
	}
	if err != nil {
		return errors.Wrap(err, "error loading profile")
	}
	cmd.Printf("Using profile %s.\n", usrProfile.ProfilePath)
	cmd.Println(`Remember to load stack environment variables using 'eval "$(elastic-package stack shellinit)"'.`)

	err = stack.BootUp(stack.Options{
		SwarmMode:    true,
		StackName:    stackName,
		StackVersion: stackVersion,
		Profile:      usrProfile,
	})
	if err != nil {
		return errors.Wrap(err, "booting up the stack failed")
	}
	cmd.Println("Done")
	return nil
}

func swarmDown(cmd *cobra.Command, args []string) error {
	cmd.Println("Shutdown the Elastic stack (on swarm)")
	stackName, err := cmd.Flags().GetString(cobraext.StackNameFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.StackNameFlagName)
	}
	err = docker.SwarmStackDown(stackName)
	if err != nil {
		return errors.Wrap(err, "booting up the stack failed")
	}
	cmd.Println("Done")
	return nil
}

func stackInit(cmd *cobra.Command, args []string) error {

	iftname, err := cmd.Flags().GetString(cobraext.InterfaceFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.InterfaceFlagDescription)
	}
	_, err = net.InterfaceByName(iftname)
	if err != nil {
		return errors.Wrap(err, "cannot create docker swarm without overlay network interface")
	}
	subnet, err := cmd.Flags().GetString(cobraext.IPSubnetFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.IPSubnetFlagName)
	}

	overlayNetworkName, err := cmd.Flags().GetString(cobraext.StackOverlayNetworkNameFlagName)
	if err != nil {
		return cobraext.FlagParsingError(err, cobraext.StackOverlayNetworkNameFlagName)
	}

	options := profile.Options{
		Name:              profile.SwarmProfile,
		FromProfile:       profile.DefaultProfile,
		OverwriteExisting: true,
	}
	err = profile.CreateProfile(options)
	if err != nil {
		return errors.Wrap(err, "swarm profile creation has failed")
	}

	joinToken, err := docker.SwarmInit(iftname)
	if err != nil {
		return errors.Wrap(err, "docker swarm creation has failed")
	}
	err = createOverlayNetwork(overlayNetworkName, subnet)
	if err != nil {
		docker.SwarmLeave()
		return errors.Wrap(err, "cannot initialize swarm")
	}
	cmd.Println(joinToken)
	return nil
}

func createOverlayNetwork(networkName, subnet string) error {
	_, _, err := net.ParseCIDR(subnet)
	if err != nil {
		return errors.Wrap(err, "create overlay network failed")
	}
	overlayArgs := []string{
		"--subnet",
		subnet,
		"--attachable",
	}
	return docker.CreateNetwork(networkName, "overlay", overlayArgs...)
}

func swarmLeave(_ *cobra.Command, _ []string) error {
	return docker.SwarmLeave()
}

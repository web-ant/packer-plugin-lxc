// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package lxc

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/user"
	"path/filepath"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

type stepExport struct{}

func (s *stepExport) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	config := state.Get("config").(*Config)
	ui := state.Get("ui").(packersdk.Ui)

	name := config.ContainerName

	lxc_dir := "/var/lib/lxc"
	user, err := user.Current()
	if err != nil {
		log.Print("Cannot find current user. Falling back to /var/lib/lxc...")
	}
	if user.Uid != "0" && user.HomeDir != "" {
		lxc_dir = filepath.Join(user.HomeDir, ".local", "share", "lxc")
	}

	containerDir := filepath.Join(lxc_dir, name)
	outputPath := filepath.Join(config.OutputDir, "rootfs.tar.gz")
	configFilePath := filepath.Join(config.OutputDir, "lxc-config")

	configFile, err := os.Create(configFilePath)

	if err != nil {
		err := fmt.Errorf("Error creating config file: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	originalConfigFile, err := os.Open(config.ConfigFile)

	if err != nil {
		err := fmt.Errorf("Error opening config file: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	_, err = io.Copy(configFile, originalConfigFile)
	if err != nil {
		err := fmt.Errorf("error copying file %s: %v", config.ConfigFile, err)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}
	commands := make([][]string, 3)
	commands[0] = []string{
		"lxc-stop", "--name", name,
	}

	// Формируем команду tar
	rootfsDir := filepath.Join(containerDir, "rootfs")
	tarArgs := []string{"-C", rootfsDir, "--numeric-owner", "--anchored"}
	for _, excl := range config.Exclude {
		ex := excl
		if len(ex) > 0 && ex[0] == '/' {
			ex = ex[1:] // убираем ведущий слэш
		}
		tarArgs = append(tarArgs, "--exclude="+ex)
	}
	tarArgs = append(tarArgs, "-czf", outputPath, ".")

	commands[1] = append([]string{"tar"}, tarArgs...)
	commands[2] = []string{
		"chmod", "+x", configFilePath,
	}

	ui.Say("Exporting container...")
	for _, command := range commands {
		err := RunCommand(command...)
		if err != nil {
			err := fmt.Errorf("Error exporting container: %s", err)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}
	}

	return multistep.ActionContinue
}

func (s *stepExport) Cleanup(state multistep.StateBag) {}

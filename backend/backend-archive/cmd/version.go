// Copyright 2023 Krisna Pranav, Sankar-2006. All rights reserved.
// Use of this source code is governed by a Apache-2.0 License
// license that can be found in the LICENSE file

package cmd

import (
	"fmt"
	"os"
	"runtime"

	"github.com/krishpranav/Mailtrix/config"
	"github.com/krishpranav/Mailtrix/utils/updater"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Display the current version & update information",
	Long:  `Display the current version & update information (if available).`,
	RunE: func(cmd *cobra.Command, args []string) error {

		updater.AllowPrereleases = true

		update, _ := cmd.Flags().GetBool("update")

		if update {
			return updateApp()
		}

		fmt.Printf("%s %s compiled with %s on %s/%s\n",
			os.Args[0], config.Version, runtime.Version(), runtime.GOOS, runtime.GOARCH)

		latest, _, _, err := updater.GithubLatest(config.Repo, config.RepoBinaryName)
		if err == nil && updater.GreaterThan(latest, config.Version) {
			fmt.Printf(
				"\nUpdate available: %s\nRun `%s version -u` to update (requires read/write access to install directory).\n",
				latest,
				os.Args[0],
			)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)

	versionCmd.Flags().
		BoolP("update", "u", false, "update to latest version")
}

func updateApp() error {
	rel, err := updater.GithubUpdate(config.Repo, config.RepoBinaryName, config.Version)
	if err != nil {
		return err
	}

	fmt.Printf("Updated %s to version %s\n", os.Args[0], rel)
	return nil
}

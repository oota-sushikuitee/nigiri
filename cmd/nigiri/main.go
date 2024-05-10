package main

import "github.com/spf13/cobra"

func main() {
	var buildCmd = &cobra.Command{
		Use:   "build",
		Short: "TODO: Add a brief description",
		Run: func(cmd *cobra.Command, args []string) {
			// Do stuff
		},
	}

	var runCmd = &cobra.Command{
		Use:   "run",
		Short: "TODO: Add a brief description",
		Run: func(cmd *cobra.Command, args []string) {
			// Do stuff
		},
	}

	var removeCmd = &cobra.Command{
		Use:   "remove",
		Short: "TODO: Add a brief description",
		Run: func(cmd *cobra.Command, args []string) {
			// Do stuff
		},
	}

	var rootCmd = &cobra.Command{
		Use:   "TODO: Add a description",
		Short: "TODO: Add a brief description",
	}

	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(removeCmd)
	rootCmd.Execute()
}

package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/nickhudkins/tokens/ccusage"
	"github.com/nickhudkins/tokens/config"
	"github.com/nickhudkins/tokens/render"
	"github.com/spf13/cobra"
)

var (
	configPath  bool
	configCache bool
)

var configCmd = &cobra.Command{
	Use:         "config",
	Short:       "Open config in $EDITOR (or print path with --path)",
	Annotations: map[string]string{"group": groupSettings},
	RunE: func(cmd *cobra.Command, args []string) error {
		if configCache {
			fmt.Println(ccusage.CachePath())
			return nil
		}
		if configPath {
			fmt.Println(config.Path())
			return nil
		}

		if err := config.EnsureExists(); err != nil {
			return err
		}
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "vi"
		}
		render.Dim.Printf("Opening %s in %s\n", config.Path(), editor)
		c := exec.Command(editor, config.Path())
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		return c.Run()
	},
}

func init() {
	configCmd.Flags().BoolVar(&configPath, "path", false, "Print the config file path")
	configCmd.Flags().BoolVar(&configCache, "cache-path", false, "Print the cache file path")
	rootCmd.AddCommand(configCmd)
}

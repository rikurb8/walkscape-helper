package cli

import (
	"path/filepath"

	"github.com/spf13/cobra"
)

type Context struct {
	JSON    bool
	DBPath  string
	Verbose bool
}

func defaultDBPath() string {
	return filepath.Join(".", "wsh.db")
}

func contextFromCommand(cmd *cobra.Command) Context {
	jsonOut, _ := cmd.Flags().GetBool("json")
	verbose, _ := cmd.Flags().GetBool("verbose")
	dbPath, _ := cmd.Flags().GetString("db-path")
	if dbPath == "" {
		dbPath = defaultDBPath()
	}

	return Context{
		JSON:    jsonOut,
		Verbose: verbose,
		DBPath:  dbPath,
	}
}

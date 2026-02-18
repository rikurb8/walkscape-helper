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
	jsonOut, err := cmd.Flags().GetBool("json")
	if err != nil {
		jsonOut = false
	}

	verbose, err := cmd.Flags().GetBool("verbose")
	if err != nil {
		verbose = false
	}

	dbPath, err := cmd.Flags().GetString("db-path")
	if err != nil {
		dbPath = ""
	}
	if dbPath == "" {
		dbPath = defaultDBPath()
	}

	return Context{
		JSON:    jsonOut,
		Verbose: verbose,
		DBPath:  dbPath,
	}
}

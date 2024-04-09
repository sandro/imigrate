package imigrate

import (
	"errors"
	"flag"
	"os"
)

// HelpText is printed when no command is specified.
const HelpText = "Please specify up, down, redo, rollback, status, or create."

// CLIErr is returned when no command is specified.
var CLIErr error = errors.New(HelpText)

// CLI parses os.Args and runs the appropriate migration command.
// Commands available are up, down, redo, rollback, status, and create.
// Most commands accept a "steps" flag which is parsed as an int. Use -steps=1
// to set it.  Up, down, and redo accept a "version" flag which is parsed as
// int64. Use --version=1610069160 to set it.
func CLI(migrator Migrator) error {
	runners := make(map[string]func())

	upCmd := flag.NewFlagSet("up", flag.ContinueOnError)
	upSteps := upCmd.Int("steps", -1, "how many migrations to execute forward")
	upVersion := upCmd.Int64("version", 0, "which version to migrate")
	runners[upCmd.Name()] = func() {
		migrator.Up(*upSteps, *upVersion)
	}

	dnCmd := flag.NewFlagSet("down", flag.ContinueOnError)
	dnSteps := dnCmd.Int("steps", -1, "how many migrations to execute backward")
	dnVersion := dnCmd.Int64("version", 0, "which version to migrate")
	runners[dnCmd.Name()] = func() {
		migrator.Down(*dnSteps, *dnVersion)
	}

	redoCmd := flag.NewFlagSet("redo", flag.ContinueOnError)
	redoSteps := redoCmd.Int("steps", 1, "how many migrations to redo")
	redoVersion := redoCmd.Int64("version", 0, "which version to migrate")
	runners[redoCmd.Name()] = func() {
		migrator.Redo(*redoSteps, *redoVersion)
	}

	rollbackCmd := flag.NewFlagSet("rollback", flag.ContinueOnError)
	rollbackSteps := rollbackCmd.Int("steps", 1, "how many migrations to rollback")
	runners[rollbackCmd.Name()] = func() {
		migrator.Rollback(*rollbackSteps)
	}

	statusCmd := flag.NewFlagSet("status", flag.ContinueOnError)
	runners[statusCmd.Name()] = func() {
		migrator.Status()
	}

	createCmd := flag.NewFlagSet("create", flag.ContinueOnError)
	runners[createCmd.Name()] = func() {
		migrator.Create(createCmd.Arg(0))
	}

	silentFlag := flag.Bool("silent", false, "Do not print messages")

	commands := []*flag.FlagSet{
		upCmd,
		dnCmd,
		redoCmd,
		rollbackCmd,
		statusCmd,
		createCmd,
	}

	if len(os.Args) < 2 {
		return CLIErr
	}

	commandFound := false
	for _, cmd := range commands {
		if os.Args[1] == cmd.Name() {
			commandFound = true
			err := cmd.Parse(os.Args[2:])
			if err == flag.ErrHelp {
				return nil
			}

			if *silentFlag {
				Logger = DiscardLogger
			}
			runners[cmd.Name()]()
		}
	}

	if !commandFound {
		return CLIErr
	}

	return nil
}

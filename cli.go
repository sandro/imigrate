package imigrate

import (
	"flag"
	"log"
	"os"
)

// HelpText is printed when no command is specified.
const HelpText = "Please specify up, down, redo, rollback, status, or create."

// CLI parses os.Args and runs the appropriate migration command.
// Commands available are up, down, redo, rollback, status, and create.
// Most commands accept a "steps" flag which is parsed as an int. Use -steps=1
// to set it.  Up, down, and redo accept a "version" flag which is parsed as
// int64. Use --version=1610069160 to set it.
func CLI(migrator Migrator) {
	upCmd := flag.NewFlagSet("up", flag.ExitOnError)
	upSteps := upCmd.Int("steps", -1, "how many migrations to execute forward")
	upVersion := upCmd.Int64("version", 0, "which version to migrate")

	dnCmd := flag.NewFlagSet("down", flag.ExitOnError)
	dnSteps := dnCmd.Int("steps", -1, "how many migrations to execute backward")
	dnVersion := dnCmd.Int64("version", 0, "which version to migrate")

	redoCmd := flag.NewFlagSet("redo", flag.ExitOnError)
	redoSteps := redoCmd.Int("steps", 1, "how many migrations to redo")
	redoVersion := redoCmd.Int64("version", 0, "which version to migrate")

	rollbackCmd := flag.NewFlagSet("rollback", flag.ExitOnError)
	rollbackSteps := rollbackCmd.Int("steps", 1, "how many migrations to rollback")

	statusCmd := flag.NewFlagSet("status", flag.ExitOnError)
	createCmd := flag.NewFlagSet("create", flag.ExitOnError)

	if len(os.Args) < 2 {
		log.Fatal(HelpText)
	}

	switch os.Args[1] {
	case "up":
		upCmd.Parse(os.Args[2:])
		migrator.Up(*upSteps, *upVersion)
	case "down":
		dnCmd.Parse(os.Args[2:])
		migrator.Down(*dnSteps, *dnVersion)
	case "redo":
		redoCmd.Parse(os.Args[2:])
		migrator.Redo(*redoSteps, *redoVersion)
	case "rollback":
		rollbackCmd.Parse(os.Args[2:])
		migrator.Rollback(*rollbackSteps)
	case "status":
		statusCmd.Parse(os.Args[2:])
		migrator.Status()
	case "create":
		createCmd.Parse(os.Args[2:])
		migrator.Create(createCmd.Arg(0))
	default:
		log.Fatal(HelpText)
	}
}

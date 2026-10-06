package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/mariusrueve/note-cli/internal/errs"
	"github.com/mariusrueve/note-cli/internal/update"
	"github.com/spf13/cobra"
)

func (a *app) updater() update.Client {
	if a.UpdateClient != nil {
		return *a.UpdateClient
	}
	return update.Client{}
}

func (a *app) executable() (string, error) {
	if a.Executable != "" {
		return a.Executable, nil
	}
	return os.Executable()
}

func (a *app) updateCommand() *cobra.Command {
	var check bool
	cmd := &cobra.Command{Use: "self-update", Short: "Check or install the latest stable GitHub release", Args: args(0, 0)}
	cmd.Flags().BoolVar(&check, "check", false, "Check availability without replacing the executable")
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		if !check && a.Distribution != "archive" {
			return fmt.Errorf("this build uses a source installer or package manager; update with that installer (use note self-update --check to check releases)")
		}
		if !check {
			if _, e := update.ParseVersion(a.Version); e != nil {
				return fmt.Errorf("development and snapshot builds use their source installer")
			}
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
		defer cancel()
		client := a.updater()
		latest, e := client.Latest(ctx)
		if e != nil {
			return e
		}
		if check {
			if update.Newer(latest, a.Version) {
				fmt.Fprintf(a.Err, "note %s is available (installed: %s).\n", latest, a.Version)
			} else if _, e := update.ParseVersion(a.Version); e != nil {
				fmt.Fprintf(a.Err, "Latest stable release: note %s (current build: %s).\n", latest, a.Version)
			} else {
				fmt.Fprintf(a.Err, "note %s is current; latest stable release: %s.\n", a.Version, latest)
			}
			return nil
		}
		if !update.Newer(latest, a.Version) {
			fmt.Fprintf(a.Err, "note %s is current; latest stable release: %s.\n", a.Version, latest)
			return nil
		}
		executable, e := a.executable()
		if e != nil {
			return e
		}
		if e = client.SelfUpdate(ctx, executable, a.Version, latest); e != nil {
			return e
		}
		fmt.Fprintf(a.Err, "Updated note %s to %s.\n", a.Version, latest)
		return nil
	}
	return cmd
}

func (a *app) installCommand() *cobra.Command {
	var directory string
	cmd := &cobra.Command{Use: "__install", Hidden: true, Args: args(0, 0)}
	cmd.Flags().StringVar(&directory, "directory", "", "Release installation directory")
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		if directory == "" {
			return errs.New(2, "installation directory is required")
		}
		if a.Distribution != "archive" {
			return fmt.Errorf("only verified release archive binaries support the release installer")
		}
		executable, e := a.executable()
		if e != nil {
			return e
		}
		path, e := update.Install(cmd.Context(), executable, directory, a.Version)
		if e != nil {
			return e
		}
		fmt.Fprintf(a.Err, "Installed note %s at %s.\n", a.Version, path)
		fmt.Fprintf(a.Err, "Add the installation directory to PATH. Update with: note self-update\n")
		return nil
	}
	return cmd
}

func (a *app) updateHint(cmd *cobra.Command) {
	if a.Distribution != "archive" || !a.Interactive() || os.Getenv("NOTE_NO_UPDATE_CHECK") != "" {
		return
	}
	switch cmd.Name() {
	case "note", "open", "new", "find", "grep", "search", "recent", "templates", "init", "sync", "doctor":
	default:
		return
	}
	for _, flag := range []string{"json", "print"} {
		if f := cmd.Flags().Lookup(flag); f != nil && f.Value.String() == "true" {
			return
		}
	}
	path, e := update.CachePath(a.Home)
	if e != nil {
		return
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), time.Second)
	defer cancel()
	if latest := a.updater().Hint(ctx, path, a.Version, time.Now()); latest != "" {
		fmt.Fprintf(a.Err, "note %s is available; run note self-update to install it.\n", latest)
	}
}

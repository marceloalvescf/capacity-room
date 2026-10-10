package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/spf13/cobra"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/audit"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/auth"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/netpolicy"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/profiles"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
	"github.com/marceloalvescf/k8s-capacity-report/internal/redact"
)

const probeTimeout = 5 * time.Second

// errUsersExist is what bootstrap-admin finds when the first user is taken.
var errUsersExist = errors.New("a user already exists; bootstrap-admin creates only the first")

// newAdminCmd builds the `admin` parent command. It has no run of its own: a
// bare `admin` shows its help.
func newAdminCmd(ctx context.Context, env Env, redactor *redact.Redactor) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "admin",
		Short: "Administer the platform.",
		// Like the root's, so a stray argument is never repeated.
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return errors.New(unknownCommandMessage(cmd))
			}
			return nil
		},
		DisableSuggestions: true,
		SilenceErrors:      true,
		SilenceUsage:       true,
		Run: func(cmd *cobra.Command, _ []string) {
			_ = cmd.Help()
		},
	}
	cmd.AddCommand(
		newBootstrapAdminCmd(ctx, env, redactor),
		newImportProfilesCmd(ctx, env, redactor),
		newMigrateCmd(ctx, env, redactor),
		newProbeCmd(ctx),
		newResetPasswordCmd(ctx, env, redactor),
	)
	return cmd
}

func newMigrateCmd(ctx context.Context, env Env, redactor *redact.Redactor) *cobra.Command {
	var config string
	cmd := &cobra.Command{
		Use:           "migrate",
		Short:         "Apply the pending database migrations.",
		Args:          noArgument("migrate"),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, pool, err := openPlatform(ctx, env, redactor, config)
			if err != nil {
				return err
			}
			defer pool.Close()
			version, err := store.Migrate(ctx, pool)
			if err != nil {
				return &exitError{1, err}
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "migrate: schema at version %d\n", version)
			return nil
		},
	}
	cmd.Flags().StringVar(&config, "config", platformConfigDefault, "Path of the TOML settings file.")
	return cmd
}

func newBootstrapAdminCmd(ctx context.Context, env Env, redactor *redact.Redactor) *cobra.Command {
	var username, language, config string
	var passwordStdin bool
	cmd := &cobra.Command{
		Use:           "bootstrap-admin",
		Short:         "Create the first administrator.",
		Args:          noArgument("bootstrap-admin"),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			name, err := bootstrapAdmin(ctx, env, redactor, bootstrapInput{
				username: username, language: language, config: config, passwordStdin: passwordStdin,
			})
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "created administrator %s\n", name)
			return nil
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&username, "username", "", "Username of the administrator.")
	flags.BoolVar(&passwordStdin, "password-stdin", false, "Read the password from one line of standard input.")
	flags.StringVar(&language, "language", "pt-BR", "Language of the administrator: pt-BR or en.")
	flags.StringVar(&config, "config", platformConfigDefault, "Path of the TOML settings file.")
	_ = cmd.MarkFlagRequired("username")
	return cmd
}

func newImportProfilesCmd(ctx context.Context, env Env, redactor *redact.Redactor) *cobra.Command {
	var file, config string
	cmd := &cobra.Command{
		Use:           "import-profiles",
		Short:         "Create or revise profiles from a TOML file.",
		Args:          noArgument("import-profiles"),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if file == "" {
				return &exitError{2, errors.New("--file is required")}
			}
			loaded, err := loadPlatformSettings(env, redactor, config)
			if err != nil {
				return err
			}
			policy := netpolicy.Policy{Hostnames: loaded.AllowedEndpointHostnames, Networks: loaded.AllowedEndpointCIDRs}
			entries, err := profiles.ReadImportFile(file, policy)
			if err != nil {
				return &exitError{2, err}
			}
			pool, err := openPool(ctx, loaded, 0)
			if err != nil {
				return err
			}
			defer pool.Close()
			if _, err := store.Migrate(ctx, pool); err != nil {
				return &exitError{1, err}
			}
			svc := &profiles.Service{
				Pool: pool, Policy: policy, Settings: loaded, Now: env.Now,
				Reveal: func(secret string) { redactor.Add(secret) },
			}
			outcomes, err := svc.Import(ctx, entries)
			if err != nil {
				return &exitError{1, err}
			}
			var out strings.Builder
			for _, o := range outcomes {
				out.WriteString(o.Action + " " + o.ID + "\n")
			}
			_, _ = io.WriteString(cmd.OutOrStdout(), out.String())
			return nil
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&file, "file", "", "Path of the TOML file of profiles.")
	flags.StringVar(&config, "config", platformConfigDefault, "Path of the TOML settings file.")
	return cmd
}

type bootstrapInput struct {
	username, language, config string
	passwordStdin              bool
}

// bootstrapAdmin creates the first user, an administrator, and returns its
// normalised username. Everything that needs no database is checked before
// the database is opened.
func bootstrapAdmin(ctx context.Context, env Env, redactor *redact.Redactor, in bootstrapInput) (string, error) {
	if env.Getenv("KCR_BOOTSTRAP_MODE") != "1" {
		return "", &exitError{1, errors.New("bootstrap-admin is off; set KCR_BOOTSTRAP_MODE=1 to run it")}
	}
	if !in.passwordStdin {
		return "", &exitError{2, errors.New("the administrator password is read from standard input; pass --password-stdin and write it there")}
	}

	name, password, err := readCredentials(env, redactor, in.username)
	if err != nil {
		return "", err
	}
	if in.language != "pt-BR" && in.language != "en" {
		return "", &exitError{2, errors.New("--language must be pt-BR or en")}
	}

	hash, err := auth.HashPassword(ctx, password)
	if err != nil {
		return "", &exitError{1, err}
	}
	rawID, err := env.NewID()
	if err != nil {
		return "", &exitError{1, err}
	}
	id, err := uuid.Parse(rawID)
	if err != nil {
		return "", &exitError{1, err}
	}

	_, pool, err := openPlatform(ctx, env, redactor, in.config)
	if err != nil {
		return "", err
	}
	defer pool.Close()
	if _, err := store.Migrate(ctx, pool); err != nil {
		return "", &exitError{1, err}
	}

	now := env.Now()
	err = store.Transact(ctx, pool, func(tx pgx.Tx) error {
		empty, err := store.LockUsersAndCheckEmpty(ctx, tx)
		if err != nil {
			return err
		}
		if !empty {
			return errUsersExist
		}
		if err := store.InsertUser(ctx, tx, store.User{
			ID: id, Username: name, PasswordHash: hash, Role: auth.RoleAdministrator,
			Language: in.language, Active: true, CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			return err
		}
		return audit.Append(ctx, tx, now, audit.Event{
			Action:     audit.ActionBootstrap,
			TargetType: "user",
			TargetID:   id.String(),
			Metadata:   audit.Metadata{"role": auth.RoleAdministrator},
		})
	})
	if err != nil {
		return "", &exitError{1, err}
	}
	return name, nil
}

func newResetPasswordCmd(ctx context.Context, env Env, redactor *redact.Redactor) *cobra.Command {
	var username, config string
	var passwordStdin bool
	cmd := &cobra.Command{
		Use:           "reset-password",
		Short:         "Set a new password for an active user.",
		Args:          noArgument("reset-password"),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			name, err := resetPassword(ctx, env, redactor, resetInput{
				username: username, config: config, passwordStdin: passwordStdin,
			})
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "reset the password of %s\n", name)
			return nil
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&username, "username", "", "Username of the user.")
	flags.BoolVar(&passwordStdin, "password-stdin", false, "Read the new password from one line of standard input.")
	flags.StringVar(&config, "config", platformConfigDefault, "Path of the TOML settings file.")
	_ = cmd.MarkFlagRequired("username")
	return cmd
}

type resetInput struct {
	username, config string
	passwordStdin    bool
}

// resetPassword sets the password of an active user and returns its
// normalised username. Everything that needs no database is checked before
// the database is opened.
func resetPassword(ctx context.Context, env Env, redactor *redact.Redactor, in resetInput) (string, error) {
	if env.Getenv("KCR_BOOTSTRAP_MODE") != "1" {
		return "", &exitError{1, errors.New("reset-password is off; set KCR_BOOTSTRAP_MODE=1 to run it")}
	}
	if !in.passwordStdin {
		return "", &exitError{2, errors.New("the new password is read from standard input; pass --password-stdin and write it there")}
	}
	name, password, err := readCredentials(env, redactor, in.username)
	if err != nil {
		return "", err
	}

	_, pool, err := openPlatform(ctx, env, redactor, in.config)
	if err != nil {
		return "", err
	}
	defer pool.Close()
	if _, err := store.Migrate(ctx, pool); err != nil {
		return "", &exitError{1, err}
	}
	if _, err := auth.ResetPassword(ctx, pool, name, password, env.Now()); err != nil {
		switch {
		case errors.Is(err, auth.ErrPasswordRaced):
			return "", &exitError{1, errors.New("the password changed during the reset; run the command again")}
		default:
			return "", &exitError{1, err}
		}
	}
	return name, nil
}

// readCredentials reads one line of standard input as a password and checks
// it, and the username, against their policies.
func readCredentials(env Env, redactor *redact.Redactor, username string) (name, password string, err error) {
	line, err := bufio.NewReader(env.Stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", "", &exitError{1, errors.New("cannot read the password from standard input")}
	}
	password = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")

	name, err = auth.ValidateUsername(username)
	if err != nil {
		return "", "", &exitError{1, errors.New("the username is not valid")}
	}
	switch err := auth.CheckPassword(password); {
	case errors.Is(err, auth.ErrPasswordTooShort):
		return "", "", &exitError{1, errors.New("the password is too short")}
	case errors.Is(err, auth.ErrPasswordTooLong):
		return "", "", &exitError{1, errors.New("the password is too long")}
	case err != nil:
		return "", "", &exitError{1, errors.New("the password is not valid")}
	}
	redactor.Add(password)
	return name, password, nil
}

func newProbeCmd(ctx context.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "probe <url>",
		Short: "Check that a URL answers 200.",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("probe takes one argument; found %d", len(args))
			}
			return nil
		},
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			target := args[0]
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
			if err != nil {
				return &exitError{1, fmt.Errorf("probe: cannot reach %s: %w", target, urlCause(err))}
			}
			resp, err := (&http.Client{Timeout: probeTimeout}).Do(req)
			if err != nil {
				return &exitError{1, fmt.Errorf("probe: cannot reach %s: %w", target, urlCause(err))}
			}
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return &exitError{1, fmt.Errorf("probe: %d", resp.StatusCode)}
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "probe: ok")
			return nil
		},
	}
}

// urlCause returns the cause inside a *url.Error, which repeats the request
// and its URL.
func urlCause(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return urlErr.Err
	}
	return err
}

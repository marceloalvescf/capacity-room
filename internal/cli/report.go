package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/marceloalvescf/k8s-capacity-report/internal/redact"
)

// reportFlags holds the destination variable of every flag of `report`. A
// zero value means "not given" only together with cmd.Flags().Changed,
// which run.go consults before trusting a field's value.
type reportFlags struct {
	config          string
	metricsURL      string
	endpointType    string
	token           string
	insecure        bool
	cluster         string
	clusterLabel    string
	clusterFilter   string
	namespaces      string
	period          string
	start           string
	end             string
	timezone        string
	nodeAutoscaling string
	minSeverity     string
	reportProfile   string
	lang            string
	output          string
	snapshotPath    string
	fromSnapshot    string
	check           bool
	verbose         bool
}

// newReportCmd builds the `report` subcommand: collect from a metrics
// endpoint and check, write a snapshot, write a PDF, or write a PDF from a
// snapshot written before.
func newReportCmd(ctx context.Context, env Env, redactor *redact.Redactor) *cobra.Command {
	f := &reportFlags{}

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Generate a capacity report from a Prometheus-compatible endpoint.",
		// A custom check, rather than cobra.NoArgs, so the message never
		// repeats a positional argument back to the terminal: one might be
		// a secret a user pasted in the wrong place.
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("report takes no argument; found %d", len(args))
			}
			return nil
		},
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if code := runReport(ctx, env, redactor, cmd, f); code != 0 {
				return &exitStatus{code: code}
			}
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&f.config, "config", "report.toml", "Path of the TOML configuration file.")
	flags.StringVar(&f.metricsURL, "metrics-url", "", "Address of the metrics endpoint.")
	flags.StringVar(&f.endpointType, "endpoint-type", "", "Endpoint kind: auto, prometheus, or grafana.")
	flags.StringVar(&f.token, "token", "", "Bearer token sent to the metrics endpoint.")
	flags.BoolVar(&f.insecure, "insecure", false, "Skip TLS certificate verification.")
	flags.StringVar(&f.cluster, "cluster", "", "Name of the cluster this report covers.")
	flags.StringVar(&f.clusterLabel, "cluster-label", "", "Label that carries the cluster name.")
	flags.StringVar(&f.clusterFilter, "cluster-filter", "", "Cluster filter mode: required, auto, or none.")
	flags.StringVar(&f.namespaces, "namespaces", "", "Comma-separated namespaces to include.")
	flags.StringVar(&f.period, "period", "", "Analysis period, such as 24h.")
	flags.StringVar(&f.start, "start", "", "Start of a calendar range, such as 2026-09-01 00:00, in the time zone.")
	flags.StringVar(&f.end, "end", "", "End of a calendar range, such as 2026-10-01 00:00, in the time zone.")
	flags.StringVar(&f.timezone, "timezone", "", "IANA time zone of the analysis.")
	flags.StringVar(&f.nodeAutoscaling, "node-autoscaling", "", "Node autoscaling: auto, enabled, or disabled.")
	flags.StringVar(&f.minSeverity, "min-severity", "", "Minimum severity shown: all, attention, or critical.")
	flags.StringVar(&f.reportProfile, "report-profile", "", "Report profile: detailed or executive.")
	flags.StringVar(&f.lang, "lang", "", "Report language: pt-BR or en.")
	flags.StringVar(&f.output, "output", "", "Path of the PDF to write.")
	flags.StringVar(&f.snapshotPath, "snapshot", "", "Path of the snapshot to write.")
	flags.StringVar(&f.fromSnapshot, "from-snapshot", "", "Path of a snapshot to render, instead of collecting.")
	flags.BoolVar(&f.check, "check", false, "Run the preflight only: endpoint, scope, capabilities.")
	flags.BoolVar(&f.verbose, "verbose", false, "Print extra diagnostic lines.")

	return cmd
}

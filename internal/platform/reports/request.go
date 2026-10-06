package reports

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"

	"github.com/jackc/pgx/v5"

	"github.com/marceloalvescf/k8s-capacity-report/internal/config"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/audit"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/auth"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/profiles"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
)

var keyPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

// checkSyntax judges a selection without reading anything. It returns one
// word per field at fault.
func checkSyntax(sel Selection) map[string]string {
	fields := map[string]string{}
	switch {
	case sel.ProfileID == "":
		fields["profile_id"] = wordMissing
	case !profiles.ValidReferenceName(sel.ProfileID):
		fields["profile_id"] = wordInvalid
	}
	if len(sel.Namespaces) == 0 {
		fields["namespaces"] = wordEmpty
	}
	seen := map[string]bool{}
	for _, ns := range sel.Namespaces {
		switch {
		case !profiles.ValidNamespace(ns):
			fields["namespaces"] = wordInvalid
		case seen[ns] && fields["namespaces"] != wordInvalid:
			fields["namespaces"] = wordRepeated
		}
		seen[ns] = true
	}
	if sel.Period != nil && config.ValidatePeriod(*sel.Period) != nil {
		fields["period"] = wordInvalid
	}
	if sel.Timezone != nil && config.ValidateTimezone(*sel.Timezone) != nil {
		fields["timezone"] = wordInvalid
	}
	if sel.ReportProfile != nil && !slices.Contains([]string{"detailed", "executive"}, *sel.ReportProfile) {
		fields["report_profile"] = wordInvalid
	}
	if sel.MinSeverity != nil && !slices.Contains([]string{"all", "attention", "critical"}, *sel.MinSeverity) {
		fields["min_severity"] = wordInvalid
	}
	if len(fields) == 0 {
		return nil
	}
	return fields
}

// resolve judges the selection against the document of the pinned revision
// and fills the defaults.
func resolve(sel Selection, d profiles.Document) (Resolved, map[string]string) {
	period, zone, reportProfile, severity := d.ResolvedDefaults()
	if sel.Period != nil {
		period = *sel.Period
	}
	if sel.Timezone != nil {
		zone = *sel.Timezone
	}
	if sel.ReportProfile != nil {
		reportProfile = *sel.ReportProfile
	}
	if sel.MinSeverity != nil {
		severity = *sel.MinSeverity
	}
	out := Resolved{
		Namespaces: slices.Clone(sel.Namespaces), Period: period, Timezone: zone,
		ReportProfile: reportProfile, MinSeverity: severity,
	}
	fields := map[string]string{}
	for _, ns := range sel.Namespaces {
		if !slices.Contains(d.AllowedNamespaces, ns) {
			fields["namespaces"] = wordNotPermitted
		}
	}
	for _, c := range []struct {
		field, value string
		permitted    []string
	}{
		{"period", period, d.PermittedPeriods},
		{"timezone", zone, d.PermittedTimezones},
		{"report_profile", reportProfile, d.PermittedReportProfiles},
		{"min_severity", severity, d.PermittedMinSeverities},
	} {
		if len(c.permitted) > 0 && !slices.Contains(c.permitted, c.value) {
			fields[c.field] = wordNotPermitted
		}
	}
	if len(fields) == 0 {
		return out, nil
	}
	return out, fields
}

// requestDigest is the SHA-256 of the selection as the operator sent it, in
// lower-case hex.
func requestDigest(sel Selection) (string, error) {
	raw, err := json.Marshal(sel)
	if err != nil {
		return "", fmt.Errorf("encode selection: %w", err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// currentRevision locks the profile row against a change and reads its
// current revision.
func currentRevision(ctx context.Context, tx pgx.Tx, id string) (store.ProfileRevision, error) {
	profile, err := store.GetProfileForShare(ctx, tx, id)
	if errors.Is(err, store.ErrNotFound) || (err == nil && profile.CurrentRevisionID == nil) {
		return store.ProfileRevision{}, &Error{Code: CodeProfileNotFound}
	}
	if err != nil {
		return store.ProfileRevision{}, fmt.Errorf("read profile: %w", err)
	}
	return store.GetProfileRevision(ctx, tx, *profile.CurrentRevisionID)
}

// Request admits a report for the operator and queues its collection. The
// same key with the same selection answers the stored report and writes
// nothing.
func (s *Service) Request(ctx context.Context, actor auth.Principal, key string, sel Selection) (Accepted, error) {
	if err := requireOperator(actor); err != nil {
		return Accepted{}, err
	}
	if !keyPattern.MatchString(key) {
		return Accepted{}, &Error{Code: CodeIdempotencyKeyInvalid}
	}
	if fields := checkSyntax(sel); fields != nil {
		return Accepted{}, &Error{Code: CodeRequestInvalid, Fields: fields}
	}
	digest, err := requestDigest(sel)
	if err != nil {
		return Accepted{}, err
	}
	now := s.now()
	var out Accepted
	err = store.Transact(ctx, s.Pool, func(tx pgx.Tx) error {
		if err := store.LockReportRequests(ctx, tx); err != nil {
			return err
		}
		storedDigest, stored, err := store.GetIdempotentReport(ctx, tx, actor.ID, key)
		switch {
		case err == nil:
			if storedDigest != digest {
				return &Error{Code: CodeIdempotencyKeyReused}
			}
			if s.afterKeyRead != nil {
				s.afterKeyRead(ctx)
			}
			// The key and its report were read together, so a sweep or a
			// purge that commits now changes neither.
			switch {
			case stored.DeletedAt != nil:
				return &Error{Code: CodeReportNotFound}
			case isExpired(stored.ExpiresAt, stored.ExpiredAt, now):
				return &Error{Code: CodeReportExpired}
			}
			out = Accepted{ID: stored.ID, Status: stored.Status, Phase: stored.Phase, ExpiresAt: stored.ExpiresAt, Replayed: true}
			return nil
		case !errors.Is(err, store.ErrNotFound):
			return err
		}

		revision, err := currentRevision(ctx, tx, sel.ProfileID)
		if err != nil {
			return err
		}
		if !revision.Active {
			return &Error{Code: CodeProfileInactive}
		}
		var document profiles.Document
		if err := json.Unmarshal(revision.Document, &document); err != nil {
			return fmt.Errorf("decode profile document: %w", err)
		}
		resolved, fields := resolve(sel, document)
		if fields != nil {
			return &Error{Code: CodeSelectionNotPermitted, Fields: fields}
		}
		global, perProfile, err := store.CountActiveReports(ctx, tx, sel.ProfileID)
		if err != nil {
			return err
		}
		switch {
		case global >= s.Settings.AdmissionLimit:
			return &Error{Code: CodeAdmissionGlobalLimit, RetryAfter: s.Settings.AdmissionRetry}
		case perProfile >= s.Settings.AdmissionProfileLimit:
			return &Error{Code: CodeAdmissionProfileLimit, RetryAfter: s.Settings.AdmissionRetry}
		}

		request, err := json.Marshal(resolved)
		if err != nil {
			return fmt.Errorf("encode resolved selection: %w", err)
		}
		id := s.newID()
		job, err := s.Queue.InsertTx(ctx, tx, CollectArgs{ReportID: id}, CollectInsertOpts(s.Settings))
		if err != nil {
			return fmt.Errorf("queue collect: %w", err)
		}
		report := store.Report{
			ID: id, OwnerID: actor.ID, ProfileID: sel.ProfileID, ProfileRevisionID: revision.ID, Request: request,
			Status: "queued", Phase: "waiting", CollectJobID: job.Job.ID,
			CreatedAt: now, ExpiresAt: now.Add(s.Settings.ReportRetention), UpdatedAt: now,
		}
		if err := store.InsertReport(ctx, tx, report); err != nil {
			return err
		}
		if err := store.InsertIdempotencyKey(ctx, tx, actor.ID, key, digest, id, now); err != nil {
			return err
		}
		if err := audit.Append(ctx, tx, now, audit.Event{
			ActorID: &actor.ID, Action: ActionReportSubmit, TargetType: targetReport, TargetID: id.String(),
			Metadata: audit.Metadata{"profile_id": sel.ProfileID, "revision_id": revision.ID.String()},
		}); err != nil {
			return err
		}
		out = Accepted{ID: id, Status: report.Status, Phase: report.Phase, ExpiresAt: report.ExpiresAt}
		return nil
	})
	if err != nil {
		return Accepted{}, err
	}
	return out, nil
}

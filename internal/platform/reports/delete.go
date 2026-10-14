package reports

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/marceloalvescf/capacity-room/internal/platform/audit"
	"github.com/marceloalvescf/capacity-room/internal/platform/auth"
	"github.com/marceloalvescf/capacity-room/internal/platform/store"
)

// ActionReportDelete is the audit action of the delete of a report.
const ActionReportDelete = "report.delete"

const maxDeleteIDs = 200

// DeleteOutcome is what a bulk delete did to one id.
type DeleteOutcome string

// The outcomes of a bulk delete.
const (
	OutcomeDeleted   DeleteOutcome = "deleted"
	OutcomeNotFound  DeleteOutcome = "not_found"
	OutcomeForbidden DeleteOutcome = "forbidden"
)

// DeleteResult is the outcome for one id.
type DeleteResult struct {
	ID      uuid.UUID
	Outcome DeleteOutcome
}

// judgeDelete says whether the actor may delete the report as read: an
// administrator any report, an operator its own, a reader none.
func judgeDelete(actor auth.Principal, r store.Report) DeleteOutcome {
	switch {
	case r.DeletedAt != nil:
		return OutcomeNotFound
	case actor.Role != auth.RoleAdministrator && r.OwnerID != actor.ID:
		return OutcomeForbidden
	}
	return OutcomeDeleted
}

// mark sets the deletion mark and writes its audit row. The mark itself
// decides whether this call deleted the report: it answers ErrNotFound when
// another call marked it first.
func mark(ctx context.Context, tx pgx.Tx, actor auth.Principal, id uuid.UUID, now time.Time) error {
	status, err := store.MarkReportDeleted(ctx, tx, id, now)
	if err != nil {
		return err
	}
	return audit.Append(ctx, tx, now, audit.Event{
		ActorID: &actor.ID, Action: ActionReportDelete, TargetType: targetReport, TargetID: id.String(),
		Metadata: audit.Metadata{"status": status},
	})
}

// Delete marks one report deleted, an expired one included: from then on it
// is absent to every read.
func (s *Service) Delete(ctx context.Context, actor auth.Principal, id uuid.UUID) error {
	now := s.now()
	return store.Transact(ctx, s.Pool, func(tx pgx.Tx) error {
		report, err := store.GetReport(ctx, tx, id)
		if errors.Is(err, store.ErrNotFound) {
			return &Error{Code: CodeReportNotFound}
		}
		if err != nil {
			return err
		}
		if report.DeletedAt != nil {
			return &Error{Code: CodeReportNotFound}
		}
		if actor.Role != auth.RoleAdministrator && (actor.Role != auth.RoleOperator || report.OwnerID != actor.ID) {
			return auth.ErrForbidden
		}
		err = mark(ctx, tx, actor, id, now)
		if errors.Is(err, store.ErrNotFound) {
			return &Error{Code: CodeReportNotFound}
		}
		return err
	})
}

// DeleteMany marks up to 200 distinct reports in one transaction and
// answers one outcome per id, in the order given.
func (s *Service) DeleteMany(ctx context.Context, actor auth.Principal, ids []uuid.UUID) ([]DeleteResult, error) {
	if actor.Role != auth.RoleOperator && actor.Role != auth.RoleAdministrator {
		return nil, auth.ErrForbidden
	}
	switch {
	case len(ids) == 0:
		return nil, &Error{Code: CodeRequestInvalid, Fields: map[string]string{"ids": wordEmpty}}
	case len(ids) > maxDeleteIDs:
		return nil, &Error{Code: CodeRequestInvalid, Fields: map[string]string{"ids": wordTooMany}}
	}
	seen := make(map[uuid.UUID]bool, len(ids))
	for _, id := range ids {
		if seen[id] {
			return nil, &Error{Code: CodeRequestInvalid, Fields: map[string]string{"ids": wordRepeated}}
		}
		seen[id] = true
	}
	now := s.now()
	results := make([]DeleteResult, len(ids))
	err := store.Transact(ctx, s.Pool, func(tx pgx.Tx) error {
		for i, id := range ids {
			results[i] = DeleteResult{ID: id, Outcome: OutcomeNotFound}
			report, err := store.GetReport(ctx, tx, id)
			if errors.Is(err, store.ErrNotFound) {
				continue
			}
			if err != nil {
				return err
			}
			results[i].Outcome = judgeDelete(actor, report)
		}
		if s.beforeMark != nil {
			s.beforeMark(ctx)
		}
		// Every call marks in the order of the ids, so two calls that share
		// ids wait for each other and never deadlock.
		order := make([]int, 0, len(ids))
		for i, r := range results {
			if r.Outcome == OutcomeDeleted {
				order = append(order, i)
			}
		}
		slices.SortFunc(order, func(a, b int) int { return slices.Compare(ids[a][:], ids[b][:]) })
		for _, i := range order {
			err := mark(ctx, tx, actor, ids[i], now)
			if errors.Is(err, store.ErrNotFound) {
				results[i].Outcome = OutcomeNotFound
				continue
			}
			if err != nil {
				return fmt.Errorf("delete report: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}

package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// AuditRow is the content of an audit event.
type AuditRow struct {
	ActorID    *uuid.UUID
	OccurredAt time.Time
	Action     string
	TargetType *string
	TargetID   *string
	Outcome    string
	Metadata   []byte // a JSON object, already checked by the audit package
}

// AuditEvent is a stored audit event.
type AuditEvent struct {
	ID int64
	AuditRow
}

// InsertAuditEvent appends r to the audit log.
func InsertAuditEvent(ctx context.Context, q Querier, r AuditRow) error {
	_, err := q.Exec(ctx, `
INSERT INTO audit_events (actor_id, occurred_at, action, target_type, target_id, outcome, metadata)
VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb)`,
		r.ActorID, r.OccurredAt, r.Action, r.TargetType, r.TargetID, r.Outcome, r.Metadata)
	if err != nil {
		return fmt.Errorf("insert audit event: %w", err)
	}
	return nil
}

// ListAuditEvents returns every row in id order; a reader for tests.
func ListAuditEvents(ctx context.Context, q Querier) ([]AuditEvent, error) {
	rows, err := q.Query(ctx, `
SELECT id, actor_id, occurred_at, action, target_type, target_id, outcome, metadata
  FROM audit_events ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list audit events: %w", err)
	}
	defer rows.Close()
	var events []AuditEvent
	for rows.Next() {
		var e AuditEvent
		if err := rows.Scan(&e.ID, &e.ActorID, &e.OccurredAt, &e.Action, &e.TargetType, &e.TargetID, &e.Outcome, &e.Metadata); err != nil {
			return nil, fmt.Errorf("list audit events: %w", err)
		}
		e.OccurredAt = e.OccurredAt.UTC()
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list audit events: %w", err)
	}
	return events, nil
}

package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
	"golang.org/x/text/cases"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
)

// The actions of this part. A later part appends its own; a constant is
// never renamed.
const (
	ActionBootstrap              = "auth.bootstrap"
	ActionLoginSucceeded         = "auth.login.succeeded"
	ActionLoginFailed            = "auth.login.failed"
	ActionLogout                 = "auth.logout"
	ActionUserCreate             = "user.create"
	ActionUserUpdate             = "user.update"
	ActionUserPasswordChanged    = "user.password_changed"
	ActionUserDelete             = "user.delete"
	ActionAccountPasswordChanged = "account.password_changed"
	ActionAccountLanguageChanged = "account.language_changed"
)

// The outcomes of an event.
const (
	OutcomeSucceeded = "succeeded"
	OutcomeFailed    = "failed"
)

// Metadata is the document of an event: a map whose values are strings,
// numbers, booleans, nil, or one more map of those.
type Metadata map[string]any

// ErrMetadata is wrapped by every refusal of Encode.
var ErrMetadata = errors.New("audit metadata refused")

var forbiddenKey = regexp.MustCompile("password|token|cookie|hash|digest|secret|dsn")

// Encode checks m and returns its JSON. It refuses a key that matches
// password|token|cookie|hash|digest|secret|dsn after case folding, a value
// of another type, and a map inside a map.
func Encode(m Metadata) ([]byte, error) {
	if len(m) == 0 {
		return []byte("{}"), nil
	}
	if err := check(m, false); err != nil {
		return nil, err
	}
	out, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMetadata, err)
	}
	return out, nil
}

func check(m map[string]any, nested bool) error {
	for k, v := range m {
		if forbiddenKey.MatchString(cases.Fold().String(k)) {
			return fmt.Errorf("%w: key %q is forbidden", ErrMetadata, k)
		}
		switch x := v.(type) {
		case nil, string, bool, int, int64, float64:
		case Metadata:
			if err := checkMap(k, x, nested); err != nil {
				return err
			}
		case map[string]any:
			if err := checkMap(k, x, nested); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%w: key %q holds a value of type %T", ErrMetadata, k, v)
		}
	}
	return nil
}

func checkMap(k string, m map[string]any, nested bool) error {
	if nested {
		return fmt.Errorf("%w: key %q holds a map inside a map", ErrMetadata, k)
	}
	return check(m, true)
}

// Event is one audit event before it is stored.
type Event struct {
	ActorID    *uuid.UUID
	Action     string
	TargetType string // empty means none
	TargetID   string // empty means none
	Outcome    string // OutcomeSucceeded when empty
	Metadata   Metadata
}

// Append encodes the metadata and inserts one row at now.
func Append(ctx context.Context, q store.Querier, now time.Time, e Event) error {
	meta, err := Encode(e.Metadata)
	if err != nil {
		return err
	}
	row := store.AuditRow{
		ActorID:    e.ActorID,
		OccurredAt: now,
		Action:     e.Action,
		Outcome:    e.Outcome,
		Metadata:   meta,
	}
	if row.Outcome == "" {
		row.Outcome = OutcomeSucceeded
	}
	if e.TargetType != "" {
		row.TargetType = &e.TargetType
	}
	if e.TargetID != "" {
		row.TargetID = &e.TargetID
	}
	return store.InsertAuditEvent(ctx, q, row)
}

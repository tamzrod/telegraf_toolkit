package metadatastore

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/registrycontract"
)

// AuditRepository persists sanitized audit events. It satisfies
// registrycontract.AuditSink. Events are append-only: there is no update or
// delete method, and each event is validated so a credential value can never be
// written (ADR-0002 audit requirement, docs/contracts/registry.md section 5).
type AuditRepository struct {
	db *sql.DB
}

// Record validates and appends an audit event.
func (r *AuditRepository) Record(event registrycontract.AuditEvent) error {
	if err := event.Validate(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO audit_events (
			actor, action, target_kind, target_id, outcome, at, correlation_id, mock
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		event.Actor, event.Action, string(event.TargetKind), event.TargetID,
		string(event.Outcome), event.At.UTC().Format(time.RFC3339Nano),
		event.CorrelationID, boolToInt(event.Mock),
	)
	if err != nil {
		return fmt.Errorf("insert audit event: %w", err)
	}
	return nil
}

// List returns every audit event in insertion order.
func (r *AuditRepository) List() ([]registrycontract.AuditEvent, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := r.db.QueryContext(ctx, `
		SELECT actor, action, target_kind, target_id, outcome, at, correlation_id, mock
		FROM audit_events ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list audit events: %w", err)
	}
	defer rows.Close()

	out := make([]registrycontract.AuditEvent, 0)
	for rows.Next() {
		var (
			event registrycontract.AuditEvent
			kind  string
			outc  string
			at    string
			mock  int
		)
		if err := rows.Scan(
			&event.Actor, &event.Action, &kind, &event.TargetID, &outc, &at,
			&event.CorrelationID, &mock,
		); err != nil {
			return nil, fmt.Errorf("scan audit event: %w", err)
		}
		event.TargetKind = registrycontract.TargetKind(kind)
		event.Outcome = registrycontract.Outcome(outc)
		event.Mock = mock != 0
		parsed, err := time.Parse(time.RFC3339Nano, at)
		if err != nil {
			return nil, fmt.Errorf("parse audit timestamp %q: %w", at, err)
		}
		event.At = parsed
		out = append(out, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate audit events: %w", err)
	}
	return out, nil
}

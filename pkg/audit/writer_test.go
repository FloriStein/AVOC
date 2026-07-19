package audit

import "testing"

// Compile-time checks (GOSTYLE-IF-03): both real implementations must keep
// satisfying the narrowed SafetyAuditWriter interface as well as the wider
// AuditWriter interface after Close was removed from both.
var (
	_ SafetyAuditWriter = (*PostgresAuditWriter)(nil)
	_ SafetyAuditWriter = (*NoopWriter)(nil)
	_ AuditWriter       = (*PostgresAuditWriter)(nil)
	_ AuditWriter       = (*NoopWriter)(nil)
)

func TestNoopWriter_WriteSync_ZeroValueEvent(t *testing.T) {
	w := NewNoopWriter()

	if err := w.WriteSync(SafetyAuditEvent{}); err != nil {
		t.Fatalf("expected nil error for zero-value event, got %v", err)
	}
}

func TestNoopWriter_QueryBySession_EmptySessionID(t *testing.T) {
	w := NewNoopWriter()

	events, err := w.QueryBySession("")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if events != nil {
		t.Fatalf("expected nil events slice, got %v", events)
	}
}

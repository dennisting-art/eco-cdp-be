package postgres

import (
	"math/big"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestNullablePgxValuesPreserveNullAndZero(t *testing.T) {
	if nullableInt64(pgtype.Int8{}) != nil {
		t.Fatal("null integer became a value")
	}
	if got := nullableInt64(pgtype.Int8{Int64: 0, Valid: true}); got == nil || *got != 0 {
		t.Fatalf("valid zero integer lost: %v", got)
	}
	if got, err := nullableFloat64(pgtype.Numeric{}); err != nil || got != nil {
		t.Fatalf("null numeric became a value: %v, %v", got, err)
	}
	if got, err := nullableFloat64(pgtype.Numeric{Int: big.NewInt(0), Valid: true}); err != nil || got == nil || *got != 0 {
		t.Fatalf("valid zero numeric lost: %v, %v", got, err)
	}
}

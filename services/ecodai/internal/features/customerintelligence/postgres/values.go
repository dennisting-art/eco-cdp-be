package postgres

import "github.com/jackc/pgx/v5/pgtype"

func nullableInt64(v pgtype.Int8) *int64 {
	if !v.Valid {
		return nil
	}
	return &v.Int64
}

func nullableFloat64(v pgtype.Numeric) (*float64, error) {
	if !v.Valid {
		return nil, nil
	}
	f, err := v.Float64Value()
	if err != nil {
		return nil, err
	}
	return &f.Float64, nil
}

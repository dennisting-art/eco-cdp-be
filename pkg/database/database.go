package database

import "github.com/jackc/pgx/v5/pgxpool"

type PostgresDB struct{ Pool *pgxpool.Pool }

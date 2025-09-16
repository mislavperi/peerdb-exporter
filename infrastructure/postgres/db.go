package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PGConnectionDetails struct {
	Host     string
	Port     int16
	Username string
	Password string
	Database string
}

func NewDBConnection(connDetails PGConnectionDetails) (*pgxpool.Pool, error) {
	return pgxpool.New(context.Background(), constructDatabaseURl(connDetails))
}

func constructDatabaseURl(connDetails PGConnectionDetails) string {
	return fmt.Sprintf("postgresql://%s:%s@%s:%d/%s", connDetails.Username, connDetails.Password, connDetails.Host, connDetails.Port, connDetails.Database)
}

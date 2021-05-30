package main

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

// sqlOpen establishes a connection to the configured PostgreSQL database.
func sqlOpen(env map[string]string) (db *sql.DB, err error) {
	if env["DB_ADAPTER"] != "postgresql" {
		err = fmt.Errorf("postgresql is the only supported DB_ADAPTER")
		return
	}

	// Greenlight's PostgreSQL has no SSL enabled as it runs within a container network.
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		env["DB_USERNAME"], env["DB_PASSWORD"],
		env["DB_HOST"], env["PORT"],
		env["DB_NAME"])

	db, err = sql.Open("postgres", connStr)
	return
}

// sqlFetchUsers lists all LDAP users with their columns from the PostgreSQL database.
func sqlFetchUsers(db *sql.DB) (uids []string, err error) {
	rows, err := db.Query("SELECT social_uid FROM users WHERE provider = 'ldap'")
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var uid string
		if err = rows.Scan(&uid); err != nil {
			return
		}
		uids = append(uids, uid)
	}

	return
}

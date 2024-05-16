// SPDX-FileCopyrightText: 2021 Alvar Penning
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	_ "github.com/lib/pq"
)

// sqlOpen establishes a connection to the configured PostgreSQL database.
func sqlOpen() (db *sql.DB, err error) {
	if os.Getenv("DB_ADAPTER") != "postgresql" {
		err = fmt.Errorf("postgresql is the only supported DB_ADAPTER")
		return
	}

	// Greenlight's PostgreSQL has no SSL enabled as it runs within a container network.
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		os.Getenv("DB_USERNAME"), os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"), os.Getenv("PORT"),
		os.Getenv("DB_NAME"))

	db, err = sql.Open("postgres", connStr)
	return
}

// sqlFetchUsers lists all LDAP users with their columns from the PostgreSQL database.
func sqlFetchUsers(db *sql.DB) (users map[string]map[string]string, err error) {
	// https://github.com/bigbluebutton/greenlight/blob/release-2.8.5/db/schema.rb#L125-L154
	// https://docs.bigbluebutton.org/greenlight/gl-config.html#ldap-auth LDAP_ATTRIBUTE_MAPPING table
	rows, err := db.Query(`
		SELECT
			name,
			email,
			external_id
		FROM
			users
		WHERE
			provider = 'greenlight'
	`)
	if err != nil {
		return
	}
	defer rows.Close()

	users = make(map[string]map[string]string)

	for rows.Next() {
		var name, email, externalId string
		if err = rows.Scan(&name, &email, &externalId); err != nil {
			return
		}

		userMap := map[string]string{
			"name":         name,
			"email":        email,
			"external_id":  externalId,
		}
		users[externalId] = userMap
	}

	return
}

// sqlUpdateUser updates the users table for all passed user attribute maps.
func sqlUpdateUser(db *sql.DB, userAttrs []map[string]string) (err error) {
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		return
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	stmt, err := tx.Prepare(`
		UPDATE
			users
		SET
			name = $1,
			email = $2,
			updated_at = NOW()
		WHERE
			external_id = $3
	`)
	if err != nil {
		return
	}
	defer stmt.Close()

	for _, userAttr := range userAttrs {
		_, err = stmt.Exec(userAttr["name"], userAttr["email"], userAttr["external_id"])
		if err != nil {
			return
		}
	}

	err = tx.Commit()
	return
}

package main

import (
	"context"
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
func sqlFetchUsers(db *sql.DB) (users map[string]map[string]string, err error) {
	// https://github.com/bigbluebutton/greenlight/blob/release-2.8.5/db/schema.rb#L125-L154
	// https://docs.bigbluebutton.org/greenlight/gl-config.html#ldap-auth LDAP_ATTRIBUTE_MAPPING table
	rows, err := db.Query(`
		SELECT
			name,
			username,
			email,
			social_uid,
			image
		FROM
			users
		WHERE
			provider = 'ldap'
	`)
	if err != nil {
		return
	}
	defer rows.Close()

	users = make(map[string]map[string]string)

	for rows.Next() {
		var name, username, email, socialUid, image string
		if err = rows.Scan(&name, &username, &email, &socialUid, &image); err != nil {
			return
		}

		userMap := map[string]string{
			"name":       name,
			"username":   username,
			"email":      email,
			"social_uid": socialUid,
			"image":      image,
		}
		users[socialUid] = userMap
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
			username = $2,
			email = $3,
			image = $4,
			updated_at = NOW()
		WHERE
			social_uid = $5
	`)
	if err != nil {
		return
	}
	defer stmt.Close()

	for _, userAttr := range userAttrs {
		_, err = stmt.Exec(userAttr["name"], userAttr["username"],
			userAttr["email"], userAttr["image"], userAttr["social_uid"])
		if err != nil {
			return
		}
	}

	err = tx.Commit()
	return
}

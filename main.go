package main

import (
	log "github.com/sirupsen/logrus"

	"github.com/joho/godotenv"
)

func main() {
	log.SetLevel(log.DebugLevel)

	env, err := godotenv.Read()
	if err != nil {
		log.WithError(err).Fatal("Cannot read the .env file")
	}

	log.WithField("env", env).Debug("Read .env")

	db, err := sqlOpen(env)
	if err != nil {
		log.WithError(err).Fatal("Cannot establish database connection")
	}
	defer db.Close()

	users, err := sqlFetchUsers(db)
	if err != nil {
		log.WithError(err).Fatal("Cannot fetch users from SQL")
	}
	log.WithField("amount", len(users)).Debug("Fetched users from SQL")

	ldap, err := ldapDial(env)
	if err != nil {
		log.WithError(err).Fatal("Cannot establish LDAP connection")
	}
	defer ldap.Close()

	for _, user := range users {
		data, err := ldapUserSearch(env, ldap, user)
		if err != nil {
			log.WithField("user", user).WithError(err).Error("Failed to query LDAP user")
			continue
		}

		log.WithFields(log.Fields{
			"user": user,
			"data": data,
		}).Info("Fetched user data from LDAP")
	}
}

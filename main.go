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

	var updateUserAttrs []map[string]string
	for user, userAttrSql := range users {
		userAttrLdap, err := ldapUserSearch(env, ldap, user)
		if err != nil {
			log.WithField("user", user).WithError(err).Error("Failed to query LDAP user")
			continue
		}

		log.WithFields(log.Fields{
			"user":      user,
			"SQL data":  userAttrSql,
			"LDAP data": userAttrLdap,
		}).Debug("Fetched user data")

		changed := false
		for attr, ldapV := range userAttrLdap {
			sqlV := userAttrSql[attr]
			if ldapV != sqlV {
				log.WithFields(log.Fields{
					"user":      user,
					"attribute": attr,
					"old":       sqlV,
					"new":       ldapV,
				}).Debug("User attribute has changed")
				changed = true
			}
		}

		if changed {
			updateUserAttrs = append(updateUserAttrs, userAttrLdap)
			log.WithField("user", user).Info("User has changed")
		}
	}

	if err = sqlUpdateUser(db, updateUserAttrs); err != nil {
		log.WithError(err).Fatal("Failed to perform SQL update")
	} else {
		log.WithField("updates", len(updateUserAttrs)).Info("Updated SQL users")
	}
}

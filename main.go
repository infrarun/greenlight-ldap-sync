package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/joho/godotenv"
)

// syncAction performs a single LDAP to PostgreSQL sync.
func syncAction() {
	log.Info("Starting LDAP sync")

	startTime := time.Now()
	defer func() {
		endTime := time.Now()
		log.WithField("time", endTime.Sub(startTime)).Info("Finished LDAP sync")
	}()

	env, err := godotenv.Read()
	if err != nil {
		log.WithError(err).Error("Cannot read the .env file")
		return
	}

	log.WithField("env", env).Debug("Read .env")

	db, err := sqlOpen(env)
	if err != nil {
		log.WithError(err).Error("Cannot establish database connection")
		return
	}
	defer db.Close()

	users, err := sqlFetchUsers(db)
	if err != nil {
		log.WithError(err).Error("Cannot fetch users from SQL")
		return
	}
	log.WithField("amount", len(users)).Debug("Fetched users from SQL")

	ldap, err := ldapDial(env)
	if err != nil {
		log.WithError(err).Error("Cannot establish LDAP connection")
		return
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

	if len(updateUserAttrs) == 0 {
		return
	}
	if err = sqlUpdateUser(db, updateUserAttrs); err != nil {
		log.WithError(err).Error("Failed to perform SQL update")
	} else {
		log.WithField("updates", len(updateUserAttrs)).Info("Updated SQL users")
	}
}

// syncInterval performs scheduled syncs based on the INTERVAL environment variable.
func syncInterval(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-ticker.C:
			syncAction()

		case <-sig:
			log.Info("Received shutdown signal")
			return
		}
	}
}

func main() {
	log.SetFormatter(&log.TextFormatter{
		DisableTimestamp:       true,
		DisableLevelTruncation: true,
		PadLevelText:           true,
	})

	if _, ok := os.LookupEnv("DEBUG"); ok {
		log.SetLevel(log.DebugLevel)
	}

	var interval time.Duration
	if intervalStr, ok := os.LookupEnv("INTERVAL"); ok {
		intervalShadow, err := time.ParseDuration(intervalStr)
		if err != nil {
			log.WithError(err).Fatal("Cannot parse INTERVAL as a Go time.Duration")
		} else if intervalShadow <= 0 {
			log.WithField("interval", intervalStr).Fatal("Negative INTERVAL value")
		}
		interval = intervalShadow
	}

	syncAction()

	if interval > 0 {
		syncInterval(interval)
	}
}

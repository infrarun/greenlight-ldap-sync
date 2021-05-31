// SPDX-FileCopyrightText: 2021 Alvar Penning
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"crypto/tls"
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/go-ldap/ldap/v3"
)

// ldapDial establishes a connection to the configured LDAP server.
func ldapDial() (conn *ldap.Conn, err error) {
	addr := fmt.Sprintf("%s:%s", os.Getenv("LDAP_SERVER"), os.Getenv("LDAP_PORT"))

	// https://github.com/bigbluebutton/greenlight/blob/release-2.8.5/app/controllers/sessions_controller.rb#L135-L140
	switch os.Getenv("LDAP_METHOD") {
	case "ssl":
		// TLS
		conn, err = ldap.DialTLS("tcp", addr, &tls.Config{})

	case "tls":
		// STARTTLS
		conn, err = ldap.Dial("tcp", addr)
		if err != nil {
			return
		}
		err = conn.StartTLS(&tls.Config{})

	default:
		// No Encryption
		conn, err = ldap.Dial("tcp", addr)
	}
	if err != nil {
		return
	}

	// https://github.com/blindsidenetworks/bn-ldap-authentication/blob/0.1.4/lib/bn-ldap-authentication.rb#L15-L32
	switch os.Getenv("LDAP_AUTH") {
	case "simple":
		// Simple Authentication, Bind DN
		err = conn.Bind(os.Getenv("LDAP_BIND_DN"), os.Getenv("LDAP_PASSWORD"))

	case "user":
		// Simple Authentication
		err = fmt.Errorf("user LDAP_AUTH is unsupported as no connection details are configured in the .env file")

	case "anonymous":
		// Anonymous Authentication
		// This case was not tested yet!
		err = conn.UnauthenticatedBind("anonymous")

	default:
		err = fmt.Errorf("%s is an unsupported LDAP_AUTH", os.Getenv("LDAP_AUTH"))
	}

	return
}

// ldapAttrMapping creates a LDAP attribute map.
//
// This logic is usually hidden from a Greenlight administrator. Internally a
// map of predefined values merged with LDAP_ATTRIBUTE_MAPPING is used to map
// LDAP attributes to an intermediate form before being mapped to Greenlight's
// SQL columns.
func ldapAttrMapping() (attrMap map[string][]string, err error) {
	// https://github.com/blindsidenetworks/bn-ldap-authentication/blob/0.1.4/lib/bn-ldap-authentication.rb#L4-L12
	attrMap = map[string][]string{
		"uid":        []string{"dn"},
		"name":       []string{"cn", "displayName"},
		"first_name": []string{"givenName"},
		"last_name":  []string{"sn"},
		"email":      []string{"mail", "email", "userPrincipalName"},
		"nickname":   []string{"uid", "userid", "sAMAccountName"},
		"image":      []string{"jpegPhoto"},
	}

	// https://github.com/blindsidenetworks/bn-ldap-authentication/blob/0.1.4/lib/bn-ldap-authentication.rb#L80-L96
	mappings := strings.Split(os.Getenv("LDAP_ATTRIBUTE_MAPPING"), ";")
	for _, mapping := range mappings {
		if mapping == "" {
			continue
		}

		kv := strings.SplitN(mapping, "=", 2)
		if len(kv) != 2 {
			err = fmt.Errorf("mapping %s cannot be split", mapping)
			return
		}

		k, v := kv[0], kv[1]

		attr, ok := attrMap[k]
		if ok {
			attr = append([]string{v}, attr...)
		} else {
			attr = []string{v}
		}

		attrMap[k] = attr

		log.WithFields(log.Fields{
			"key":    k,
			"values": attr,
		}).Debug("Updated attribute map based on LDAP_ATTRIBUTE_MAPPING")
	}

	return
}

// ldapAttrFlatten reduces the ldapAttrMapping map to a value slice.
func ldapAttrFlatten(attrMap map[string][]string) (ldapAttrs []string) {
	for _, vs := range attrMap {
		ldapAttrs = append(ldapAttrs, vs...)
	}
	return
}

// ldapUserSearch returns a map of this user's attributes based on the .env file.
func ldapUserSearch(conn *ldap.Conn, user string) (ldapAttrs map[string]string, err error) {
	attrMap, err := ldapAttrMapping()
	if err != nil {
		return
	}

	searchReq := ldap.NewSearchRequest(
		os.Getenv("LDAP_BASE"),
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0,
		false,
		fmt.Sprintf("(&(%s=%s)%s)", os.Getenv("LDAP_UID"), user, os.Getenv("LDAP_FILTER")),
		ldapAttrFlatten(attrMap),
		nil)

	searchResp, err := conn.Search(searchReq)
	if err != nil {
		return
	}

	if l := len(searchResp.Entries); l != 1 {
		err = fmt.Errorf("expected exactly one LDAP response, got %d", l)
		return
	}

	// https://docs.bigbluebutton.org/greenlight/gl-config.html#ldap-auth LDAP_ATTRIBUTE_MAPPING table
	greenlightMap := map[string]string{
		"uid":      "social_uid",
		"name":     "name",
		"email":    "email",
		"nickname": "username",
		"image":    "image",
	}

	// Create map with key: LDAP key -> intermediate key -> Greenlight key
	ldapAttrs = make(map[string]string)
	for attrMapK, attrMapVs := range attrMap {
		// Find an intermediate key for each attrMap key.
		var attrValue string
	LoopAttrMapVs:
		for _, attrMapV := range attrMapVs {
			for _, attr := range searchResp.Entries[0].Attributes {
				if attrMapV == attr.Name {
					attrValue = strings.Join(attr.Values, " ")
					break LoopAttrMapVs
				}
			}
		}

		// Ignore unset attrMap keys, e.g., image resp. jpegPhoto
		if attrValue == "" {
			log.WithFields(log.Fields{
				"user":                   user,
				"intermediate attribute": attrMapK,
			}).Debug("Cannot find LDAP attribute for intermediate attribute mapping")
			continue
		}

		// Map intermediate key to a Greenlight database key, only if existent
		if dbKey, ok := greenlightMap[attrMapK]; ok {
			ldapAttrs[dbKey] = attrValue
		} else {
			log.WithFields(log.Fields{
				"user":                   user,
				"intermediate attribute": attrMapK,
			}).Debug("Cannot map intermediate attribute to Greenlight attribute")
		}
	}

	return
}

<!--
SPDX-FileCopyrightText: 2021 Alvar Penning

SPDX-License-Identifier: GPL-3.0-or-later
-->

# `greenlight-ldap-sync`

The [Greenlight][greenlight] web front-end for a BigBlueButton server allows an LDAP-based user authentication by following the [LDAP Auth][greenlight-ldap-auth] documentation section.

However, the user data is synchronized only upon first login from the LDAP to Greenlight's PostgreSQL database.
Later synchronizations are currently not possible, as discussed in [issue #1918][greenlight-issue-1918].
Those might be necessary if, for example, a user's name changes.

This tool, `greenlight-ldap-sync`, addresses this issue by performing a resync based on the already existing `.env` configuration file.
It is designed to be easily integrated into a default Docker Compose-based installation.


## Usage

The entire program is configured via environment variables.
These are those from Greenlight's `.env` file plus the following ones:

- `SYNC_DEBUG`:
  If this environment variable is set, logging is strongly amplified.
  This log contains sensitive data and should only be activated for debugging purposes!
- `SYNC_INTERVAL`:
  If this environment variable is set, the sync is executed routinely.
  The value of the variable corresponds to the time interval between the syncs, specified as duration string for Go's [`time.ParseDuration`][golang-time-parseduration] function:

  > A duration string is a […] sequence of decimal numbers, each with optional fraction and a unit suffix, such as "300ms" […] or "2h45m".
  > Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h".


## Deployment

The installation is done by adding this repository to the existing Greenlight installation and customizing the `docker-compose.yml` file.

```sh
# Change to your Greenlight directory, /opt/greenlight for me
cd /opt/greenlight

# Clone this repository within your greenlight directory
git clone https://github.com/infrarun/greenlight-ldap-sync.git
```

Edit Greenlight's `docker-compose.yml` file and append a new `service` to the file.
The following example would perform a sync every hour.

```
  ldap-sync:
    build:
      context: ./greenlight-ldap-sync
    env_file: .env
    environment:
      - SYNC_INTERVAL=1h
    restart: unless-stopped
    links:
      - db
```

Finally, you need to restart Docker Compose.
The initial start with the new container might take a while, as it needs to be built first.


## Development

As `greenlight-ldap-sync` tries to honor Greenlight's `.env` file, it should be copied to this directory.

By default, Greenlight's PostgreSQL database daemon is only reachable within the Docker network.
However, one can tunnel the PostgreSQL port to the development machine via SSH.

```
# Fetch container's IP address on the BBB host
user@bbb:~$ sudo docker inspect -f '{{ .NetworkSettings.IPAddress }}' greenlight_db_1
172.17.0.2

# Reconnect and bind the container's port locally
user@local:~$ ssh -L 5432:172.17.0.2:5432 bbb
```

Afterwards, the `DB_HOST` variable within the local `.env` file should be altered to `DB_HOST=localhost`.

Since the deployment is realized via Docker Compose, a Docker container can also be used for development.
The necessary environment variables both from the `.env` file as well as those for `greenlight-ldap-sync` can be passed via command line arguments.

```sh
docker build -t greenlight-ldap-sync .

docker run --rm \
  --env-file .env \
  --env SYNC_DEBUG=on \
  --env SYNC_INTERVAL=10s \
  --network=host \
  greenlight-ldap-sync
```


## License

GNU GPLv3 or later.


[golang-time-parseduration]: https://golang.org/pkg/time/#ParseDuration
[greenlight-issue-1918]: https://github.com/bigbluebutton/greenlight/issues/1918
[greenlight-ldap-auth]: https://docs.bigbluebutton.org/greenlight/gl-config.html#ldap-auth
[greenlight]: https://github.com/bigbluebutton/greenlight

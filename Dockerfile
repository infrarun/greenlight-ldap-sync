# SPDX-FileCopyrightText: 2021 Alvar Penning
#
# SPDX-License-Identifier: GPL-3.0-or-later

FROM golang:1.22 AS builder

WORKDIR /go/src/greenlight-ldap-sync
COPY . .

RUN CGO_ENABLED=0 go build -tags netgo -o /greenlight-ldap-sync


FROM alpine:3.13

RUN apk --no-cache add ca-certificates
COPY --from=builder /greenlight-ldap-sync /bin/greenlight-ldap-sync

RUN adduser -G users -S -H user
USER user

CMD ["/bin/greenlight-ldap-sync"]

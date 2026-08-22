#!/bin/sh
set -eu
: "${QAAT_HOST:=qaat.orion13.us}"
: "${QAAT_PORTAL_HOST:=students.orion13.us}"
sed -e "s/__QAAT_HOST__/${QAAT_HOST}/g" \
    -e "s/__QAAT_PORTAL_HOST__/${QAAT_PORTAL_HOST}/g" \
    /etc/nginx/qaat.conf.template > /etc/nginx/conf.d/default.conf
exec nginx -g 'daemon off;'

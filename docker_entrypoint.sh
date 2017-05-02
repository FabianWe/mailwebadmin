#!/bin/bash

# MIT License
# Copyright (c) 2017 Fabian Wenzelmann
#
# Permission is hereby granted, free of charge, to any person obtaining a copy of
# this software and associated documentation files (the "Software"), to deal in
# the Software without restriction, including without limitation the rights to use,
# copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the
# Software, and to permit persons to whom the Software is furnished to do so,
# subject to the following conditions:
#
# The above copyright notice and this permission notice shall be included in all
# copies or substantial portions of the Software.
#
# THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
# IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
# FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
# COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
# IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
# CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

set -e

: ${DB_HOST:=mysql}
CONFIG="/config/mailconf"

if [ ! -f "$CONFIG" ]; then
  touch "$CONFIG"
  if [ ! -z "$DELETE_DIRS" ]; then
    printf "delete = %s\n" "$DELETE_DIRS" >> "$CONFIG"
  fi
  printf "backup = \"/backup/\"\n" >> "$CONFIG"
  if [ ! -z "$ADMIN_USER" ]; then
    if [ -z "$ADMIN_PASSWORD" ]; then
      printf "admin user set but admin password not, error!\n"
      rm "$CONFIG"
      exit 1
    fi
    printf "admin_user = \"%s\"\nadmin_password = \"%s\"\n" "$ADMIN_USER" "$ADMIN_PASSWORD" >> "$CONFIG"
  fi

  # mysql part
  printf "\n[mysql]\n" >> "$CONFIG"
  printf "host = \"%s\"\n" "$DB_HOST" >> "$CONFIG"
  if [ ! -z "$DB_PASSWORD" ]; then
    printf "password = \"%s\"\n" "$DB_PASSWORD" >> "$CONFIG"
  fi

  if [ ! -z "$DB_NAME" ]; then
    printf "dbname = \"%s\"\n" "$DB_NAME" >> "$CONFIG"
  fi

  if [ ! -z "$DB_PORT" ]; then
    printf "port = %s\n" "$DB_PORT" >> "$CONFIG"
  fi

  # timers part
  printf "\n[timers]\n" >> "$CONFIG"
  if [ ! -z "$SESSION_LIFESPAN" ]; then
    printf "session_lifespan = \"%s\"\n" "$SESSION_LIFESPAN" >> "$CONFIG"
  fi
  if [ ! -z "$INVALID_KEYS_TIMER" ]; then
    printf "invalid_keys = \"%s\"\n" "$INVALID_KEYS_TIMER" >> "$CONFIG"
  fi
fi

exec "$@"

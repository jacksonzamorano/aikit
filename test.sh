#!/bin/bash
# scripts/test.sh
set -a
source .env
set +a
go test $@

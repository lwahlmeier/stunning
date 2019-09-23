#!/usr/bin/dumb-init /bin/bash

DEFAULT_STUNNING_LOGLEVEL="info"
DEFAULT_STUNNING_STUNADDRESS="0.0.0.0:3478"
DEFAULT_STUNNING_METRICSADDRESS="0.0.0.0:80"
DEFAULT_STUNNING_POOLSIZE="25"
DEFAULT_STUNNING_LOGGEOIP="false"

. /env.sh

for var in ${!DEFAULT_STUNNING*}; do
  t=${var/DEFAULT_/}
  if [ -z ${!t} ]; then
    echo "Using default for ${t}:${!var}"
    eval ${t}=${!var}
    export "${t}"
  else
    echo "Using override value for ${t}"
  fi
done


exec "${@}"


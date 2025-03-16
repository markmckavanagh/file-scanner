#!/bin/bash

CLAMAV_HOST="clamav"
CLAMAV_PORT="3310"

# Maximum wait time
MAX_WAIT=120
SECONDS_WAITED=0

while [ $SECONDS_WAITED -lt $MAX_WAIT ]; do
    # Try to send a ping to ClamAV to see if it's ready
    if echo "PING" | nc -w 1 $CLAMAV_HOST $CLAMAV_PORT | grep "PONG"; then
        echo "ClamAV is ready!"
        exit 0
    fi
    
    echo "Waiting for ClamAV to be ready..."
    sleep 5
    SECONDS_WAITED=$((SECONDS_WAITED + 5))
done

echo "ClamAV did not become ready in time"
exit 1

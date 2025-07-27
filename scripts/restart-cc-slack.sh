#!/bin/bash

# Restart cc-slack through the manager
echo "🔄 Requesting cc-slack restart..."

response=$(curl -s -w "\n%{http_code}" -X POST http://localhost:10080/restart)
http_code=$(echo "$response" | tail -n1)
json_response=$(echo "$response" | head -n-1)

if [ "$http_code" = "200" ]; then
    echo "✅ Restart completed successfully!"
    
    # Try to extract PID from JSON response
    pid=$(echo "$json_response" | grep -o '"pid":[0-9]*' | grep -o '[0-9]*')
    duration=$(echo "$json_response" | grep -o '"duration":"[^"]*"' | cut -d'"' -f4)
    
    if [ -n "$pid" ]; then
        echo "   New PID: $pid"
    fi
    if [ -n "$duration" ]; then
        echo "   Duration: $duration"
    fi
elif [ "$http_code" = "500" ]; then
    echo "❌ Restart failed!"
    error=$(echo "$json_response" | grep -o '"error":"[^"]*"' | cut -d'"' -f4)
    if [ -n "$error" ]; then
        echo "   Error: $error"
    fi
    exit 1
else
    echo "❌ Failed to connect to manager (HTTP $http_code)"
    echo "   Make sure the manager is running on port 10080"
    exit 1
fi
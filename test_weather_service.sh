#!/bin/bash

# Test script for weather service
# This script sends multiple requests to test request aggregation functionality

# Set the base URL - change this to match your service's address
BASE_URL="http://localhost:3000"

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${BLUE}Weather Service Test Script${NC}"
echo "This script will test the request aggregation functionality"
echo "Make sure your service is running before proceeding"
echo ""

# Test 1: Single request
echo -e "${YELLOW}Test 1: Single request for Istanbul${NC}"
echo "This should take about 6 seconds (5s wait + 1s API call)"
time curl -s "${BASE_URL}/weather?q=Istanbul" | jq .
echo ""

# Test 2: Multiple requests for the same location within 5 seconds
echo -e "${YELLOW}Test 2: Multiple requests for London within 5 seconds${NC}"
echo "Sending 5 requests with 1 second intervals"
echo "The first request should take about 5-6 seconds, but subsequent ones should be faster"

for i in {1..5}; do
  echo -e "${GREEN}Request $i:${NC}"
  time curl -s "${BASE_URL}/weather?q=London" | jq . &
  sleep 1
done

# Wait for all background processes to complete
wait
echo ""

# Test 3: Reaching the queue limit (10 requests)
echo -e "${YELLOW}Test 3: Reaching the queue limit (10 requests) for Paris${NC}"
echo "Sending 10 requests simultaneously"
echo "All requests should be processed immediately once the 10th request arrives"

for i in {1..10}; do
  echo -e "${GREEN}Request $i:${NC}"
  time curl -s "${BASE_URL}/weather?q=Paris" | jq . &
  # No sleep here - we want to send them as quickly as possible
done

# Wait for all background processes to complete
wait
echo ""

# Test 4: Different locations
echo -e "${YELLOW}Test 4: Different locations${NC}"
echo "Sending requests for different locations"
echo "Each location should have its own queue"

LOCATIONS=("New York" "Tokyo" "Berlin" "Sydney" "Moscow")

for location in "${LOCATIONS[@]}"; do
  echo -e "${GREEN}Request for $location:${NC}"
  time curl -s "${BASE_URL}/weather?q=${location// /%20}" | jq . &
  sleep 0.5
done

# Wait for all background processes to complete
wait
echo ""

# Test 5: Requests after processing has started
echo -e "${YELLOW}Test 5: Requests after processing has started${NC}"
echo "Sending a request for Rome, waiting 3 seconds, then sending another"
echo "The second request should join the first if it's still within the 5-second window"

echo -e "${GREEN}First request for Rome:${NC}"
time curl -s "${BASE_URL}/weather?q=Rome" | jq . &

sleep 3

echo -e "${GREEN}Second request for Rome (after 3s):${NC}"
time curl -s "${BASE_URL}/weather?q=Rome" | jq . &

# Wait for all background processes to complete
wait
echo ""

echo -e "${BLUE}All tests completed!${NC}"
echo "Check the response times to verify that request aggregation is working correctly."
echo "The database should also contain logs of all these queries."

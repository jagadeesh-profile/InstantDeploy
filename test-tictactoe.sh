#!/bin/bash



set -e



echo " Testing TicTacToe Deployment in Isolated Environment"

echo "========================================================"



# Create deployment

echo "Creating deployment..."

RESPONSE=$(curl -s -X POST http://localhost:8082/api/v1/deployments \

  -H "Content-Type: application/json" \

  -d '{

    "repository": "https://github.com/voiko/TicTacToe_Application_mission_1.git",

    "branch": "main",

    "cpu_limit": "2.0",

    "memory_limit": "1g"

  }')



DEPLOYMENT_ID=$(echo $RESPONSE | jq -r '.id')



if [ "$DEPLOYMENT_ID" = "null" ]; then

    echo " Failed to create deployment"

    echo "Response: $RESPONSE"

    exit 1

fi



echo " Deployment created: $DEPLOYMENT_ID"



# Monitor status

echo ""

echo "Monitoring deployment (max 5 minutes)..."

MAX_WAIT=300

ELAPSED=0



while [ $ELAPSED -lt $MAX_WAIT ]; do

    STATUS=$(curl -s http://localhost:8082/api/v1/deployments/${DEPLOYMENT_ID} | jq -r '.status')

    echo " Status: $STATUS (${ELAPSED}s elapsed)"

    

    if [ "$STATUS" = "running" ]; then

        echo " Deployment is running!"

        

        # Get URL

        LOCAL_URL=$(curl -s http://localhost:8082/api/v1/deployments/${DEPLOYMENT_ID} | jq -r '.local_url')

        echo ""

        echo " TicTacToe is ready!"

        echo "Access at: $LOCAL_URL"

        echo ""

        echo "Opening in browser..."

        

        # Open browser (cross-platform)

        if command -v open > /dev/null; then

            open $LOCAL_URL

        elif command -v xdg-open > /dev/null; then

            xdg-open $LOCAL_URL

        else

            echo "Please open: $LOCAL_URL"

        fi

        

        exit 0

    elif [ "$STATUS" = "failed" ]; then

        echo " Deployment failed"

        curl -s http://localhost:8082/api/v1/deployments/${DEPLOYMENT_ID}/logs | jq -r '.[-10:] | .[] | .message'

        exit 1

    fi

    

    sleep 5

    ELAPSED=$((ELAPSED + 5))

done



echo " Deployment timed out"

exit 1





================================================================================

PART 6: WINDOWS-SPECIFIC SETUP (PowerShell Version)

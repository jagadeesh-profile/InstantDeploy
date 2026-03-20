import ws from 'k6/ws';
import { check, group, sleep } from 'k6';
import { Counter } from 'k6/metrics';

// Custom metrics
const wsConnErrors = new Counter('ws_connect_errors');
const wsMessageLatency = new Counter('ws_message_latency');

export const options = {
  stages: [
    { duration: '1m', target: 10 },   // Ramp up to 10 connections
    { duration: '3m', target: 50 },   // Ramp up to 50 connections
    { duration: '3m', target: 100 },  // Ramp up to 100 connections
    { duration: '2m', target: 50 },   // Ramp down to 50
    { duration: '1m', target: 0 },    // Ramp down to 0
  ],
  thresholds: {
    'ws_connect_errors': ['count<10'],
    'ws_message_latency': ['p(95)<100', 'p(99)<500'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'ws://localhost:8080';
const WS_URL = `${BASE_URL}/api/v1/ws`;

export default function () {
  const userId = `user_${__VU}`;
  const token = generateToken(userId);  // In real scenario, acquire valid token

  group('WebSocket Connection', () => {
    const url = `${WS_URL}?token=${token}`;

    const res = ws.connect(url, (socket) => {
      // Connection established
      check(socket, {
        'text status 101': (r) => r && r.status === 101,
        'connection open': (r) => r && r.status === 0,
      });

      // Set listeners
      socket.on('open', () => {
        console.log(`VU ${__VU}: Connected`);
      });

      socket.on('message', (data) => {
        const message = JSON.parse(data);
        console.log(`VU ${__VU}: Received message - ${message.type}`);
      });

      socket.on('close', () => {
        console.log(`VU ${__VU}: Connection closed`);
      });

      socket.on('error', (e) => {
        console.error(`VU ${__VU}: Error - ${e}`);
        wsConnErrors.add(1);
      });

      // Subscribe to deployments
      socket.send(JSON.stringify({
        type: 'subscribe',
        channel: 'deployments',
        userId: userId,
      }));

      sleep(2);

      // Send deployment updates request
      socket.send(JSON.stringify({
        type: 'get_deployments',
        userId: userId,
        limit: 10,
      }));

      sleep(1);

      // Listen for deployment status updates (3 min connection)
      const connectionTime = Date.now();
      while (Date.now() - connectionTime < 180000) {
        socket.send(JSON.stringify({
          type: 'ping',
          timestamp: Date.now(),
        }));

        sleep(30);  // Send ping every 30 seconds

        // Simulate checking for messages
        const beforeCheck = Date.now();
        // In real scenario, measure actual message receive latency
        wsMessageLatency.add(Date.now() - beforeCheck);
      }

      socket.close();
    });

    check(res, {
      'ws connection established': (r) => r && r.status === 101,
    });
  });

  // Test connection pool exhaustion
  group('Concurrent Connections', () => {
    const promises = [];
    
    for (let i = 0; i < 5; i++) {
      const url = `${WS_URL}?token=${token}&client=${i}`;
      
      ws.connect(url, (socket) => {
        check(socket, {
          'concurrent connection established': (r) => r && r.status === 101,
        });

        socket.send(JSON.stringify({
          type: 'subscribe',
          channel: 'events',
        }));

        sleep(10);
        socket.close();
      });
    }
  });

  sleep(5);
}

// Utility function (in real scenario, get from auth endpoint)
function generateToken(userId) {
  // Placeholder - in real test, obtain valid JWT from auth endpoint
  return `token_${userId}_${Date.now()}`;
}

// Cleanup
export function teardown() {
  console.log('WebSocket stress test completed');
}

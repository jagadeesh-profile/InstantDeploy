import http from 'k6/http';
import { check, group, sleep } from 'k6';

// Performance test configuration
export const options = {
  // Stage 1: Warm up
  stages: [
    { duration: '1m', target: 10 },   // Ramp up to 10 users
    { duration: '3m', target: 50 },   // Ramp up to 50 users
    { duration: '5m', target: 100 },  // Ramp up to 100 users
    { duration: '5m', target: 100 },  // Stay at 100 users
    { duration: '3m', target: 50 },   // Ramp down to 50 users
    { duration: '1m', target: 0 },    // Ramp down to 0 users
  ],
  
  // Thresholds - Define performance acceptance criteria
  thresholds: {
    'http_req_duration': ['p(95)<500', 'p(99)<1000'], // 95% under 500ms, 99% under 1s
    'http_req_failed': ['rate<0.1'],                   // Error rate < 10%
    'http_reqs': ['rate>10'],                          // At least 10 requests/sec
    checks: ['rate>0.99'],                             // 99% check pass rate
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const API_URL = `${BASE_URL}/api/v1`;

export default function () {
  // User authentication
  group('Authentication', () => {
    let username = `user_${__VU}_${__ITER}`;
    let email = `${username}@test.local`;
    let password = 'Test123!Pass456';

    // Sign up
    let signupRes = http.post(`${API_URL}/auth/signup`, {
      username: username,
      email: email,
      password: password,
    });

    check(signupRes, {
      'signup status 200': (r) => r.status === 200,
      'signup returns verification_code': (r) => r.json('verification_code') !== '',
    });

    let verificationCode = signupRes.json('verification_code');
    sleep(1);

    // Verify user
    let verifyRes = http.post(`${API_URL}/auth/verify`, {
      username: username,
      code: verificationCode,
    });

    check(verifyRes, {
      'verify status 200': (r) => r.status === 200,
    });
    sleep(1);

    // Login
    let loginRes = http.post(`${API_URL}/auth/login`, {
      username: username,
      password: password,
    });

    check(loginRes, {
      'login status 200': (r) => r.status === 200,
      'login returns token': (r) => r.json('token') !== '',
    });

    let token = loginRes.json('token');

    // Repository search
    group('Repository Operations', () => {
      let searchRes = http.get(
        `${API_URL}/repositories?query=github.com`,
        {
          headers: {
            Authorization: `Bearer ${token}`,
          },
        }
      );

      check(searchRes, {
        'search status 200': (r) => r.status === 200,
        'search returns repositories': (r) => r.json('repositories').length > 0,
      });
      sleep(1);
    });

    // Deployment operations
    group('Deployment Operations', () => {
      let deployRes = http.post(
        `${API_URL}/deployments`,
        {
          repository: 'vercel/ms',
          branch: 'main',
          url: '',
        },
        {
          headers: {
            Authorization: `Bearer ${token}`,
          },
        }
      );

      check(deployRes, {
        'deployment creation status 200': (r) => r.status === 200,
        'deployment has id': (r) => r.json('id') !== '',
      });

      let deploymentId = deployRes.json('id');
      sleep(1);

      // Check deployment status (multiple times to simulate polling)
      for (let i = 0; i < 5; i++) {
        let statusRes = http.get(
          `${API_URL}/deployments/${deploymentId}/status`,
          {
            headers: {
              Authorization: `Bearer ${token}`,
            },
          }
        );

        check(statusRes, {
          'status check 200': (r) => r.status === 200,
          'status has status field': (r) => r.json('status') !== '',
        });

        sleep(2);

        // Break if deployment finished
        let status = statusRes.json('status');
        if (status === 'running' || status === 'failed') {
          break;
        }
      }

      // Get deployment logs
      let logsRes = http.get(
        `${API_URL}/deployments/${deploymentId}/logs`,
        {
          headers: {
            Authorization: `Bearer ${token}`,
          },
        }
      );

      check(logsRes, {
        'logs status 200': (r) => r.status === 200,
      });
      sleep(1);

      // List deployments
      let listRes = http.get(
        `${API_URL}/deployments`,
        {
          headers: {
            Authorization: `Bearer ${token}`,
          },
        }
      );

      check(listRes, {
        'list status 200': (r) => r.status === 200,
        'list returns deployments': (r) => r.json('deployments').length >= 0,
      });
      sleep(1);

      // Delete deployment
      let deleteRes = http.del(
        `${API_URL}/deployments/${deploymentId}`,
        null,
        {
          headers: {
            Authorization: `Bearer ${token}`,
          },
        }
      );

      check(deleteRes, {
        'delete status 200': (r) => r.status === 200 || r.status === 204,
      });
    });

    // Health check
    group('Health Checks', () => {
      let healthRes = http.get(`${API_URL}/health`);
      check(healthRes, {
        'health status 200': (r) => r.status === 200,
        'health response ok': (r) => r.json('status') === 'ok',
      });
    });

    sleep(5);
  });
}

// Custom metrics
export function handleSummary(data) {
  return {
    'stdout': textSummary(data, { indent: ' ', enableColors: true }),
    'summary.json': JSON.stringify(data),
  };
}

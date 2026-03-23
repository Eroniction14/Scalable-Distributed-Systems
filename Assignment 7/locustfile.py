"""
Locust load tests for HW7.

Usage:
  Normal sync test (5 users, 30s):
    locust -f locustfile.py SyncUser --headless -u 5 -r 1 -t 30s --host http://<ALB_DNS>

  Flash sale sync test (20 users, 60s):
    locust -f locustfile.py SyncUser --headless -u 20 -r 10 -t 60s --host http://<ALB_DNS>

  Flash sale async test (20 users, 60s):
    locust -f locustfile.py AsyncUser --headless -u 20 -r 10 -t 60s --host http://<ALB_DNS>
"""

import random
from locust import HttpUser, task, between


ORDER_PAYLOAD = {
    "customer_id": 0,      # will be randomized per request
    "items": [
        {"product_id": "FLASH-001", "quantity": 1, "price": 29.99},
        {"product_id": "FLASH-002", "quantity": 2, "price": 9.99},
    ],
}


class SyncUser(HttpUser):
    """Hits the synchronous endpoint."""
    wait_time = between(0.1, 0.5)   # 100-500ms between requests

    @task
    def place_order_sync(self):
        payload = {**ORDER_PAYLOAD, "customer_id": random.randint(1, 10000)}
        self.client.post("/orders/sync", json=payload)


class AsyncUser(HttpUser):
    """Hits the asynchronous endpoint."""
    wait_time = between(0.1, 0.5)

    @task
    def place_order_async(self):
        payload = {**ORDER_PAYLOAD, "customer_id": random.randint(1, 10000)}
        self.client.post("/orders/async", json=payload)
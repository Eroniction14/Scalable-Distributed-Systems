from locust import HttpUser, FastHttpUser, task, between
import json
import random

# ---- Standard HttpUser ----
class ProductUser(HttpUser):
    wait_time = between(1, 3)
    product_counter = 0

    @task(3)  # GET is 3x more common (realistic: reads > writes)
    def get_product(self):
        product_id = random.randint(1, max(1, ProductUser.product_counter))
        self.client.get(f"/products/{product_id}", name="/products/[id]")

    @task(1)
    def create_product(self):
        ProductUser.product_counter += 1
        pid = ProductUser.product_counter
        payload = {
            "product_id": pid,
            "sku": f"SKU-{pid}",
            "manufacturer": f"Manufacturer-{pid}",
            "category_id": random.randint(1, 10),
            "weight": random.randint(100, 5000),
            "some_other_id": random.randint(1, 100)
        }
        self.client.post(
            f"/products/{pid}/details",
            json=payload,
            name="/products/[id]/details"
        )


# ---- FastHttpUser (for comparison) ----
class FastProductUser(FastHttpUser):
    wait_time = between(1, 3)
    product_counter = 0

    @task(3)
    def get_product(self):
        product_id = random.randint(1, max(1, FastProductUser.product_counter))
        self.client.get(f"/products/{product_id}", name="/products/[id]")

    @task(1)
    def create_product(self):
        FastProductUser.product_counter += 1
        pid = FastProductUser.product_counter
        payload = {
            "product_id": pid,
            "sku": f"SKU-{pid}",
            "manufacturer": f"Manufacturer-{pid}",
            "category_id": random.randint(1, 10),
            "weight": random.randint(100, 5000),
            "some_other_id": random.randint(1, 100)
        }
        self.client.post(
            f"/products/{pid}/details",
            data=json.dumps(payload),
            headers={"Content-Type": "application/json"},
            name="/products/[id]/details"
        )
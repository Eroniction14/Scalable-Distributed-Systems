from locust import FastHttpUser, task
import random

class SearchUser(FastHttpUser):
    @task
    def search(self):
        q = random.choice(["alpha", "beta", "electronics", "books", "home"])
        self.client.get(f"/products/search?q={q}")
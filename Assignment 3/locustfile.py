from locust import task, between
from locust.contrib.fasthttp import FastHttpUser

class AlbumUser(FastHttpUser):
    wait_time = between(1, 2)

    @task(3)
    def get_albums(self):
        self.client.get("/albums")

    @task(1)
    def post_album(self):
        self.client.post("/albums", json={
            "id": "99",
            "title": "Test Album",
            "artist": "Test Artist",
            "price": 9.99
        })
from django.contrib.auth import get_user_model
from rest_framework.test import APITestCase


class LoginTest(APITestCase):
    """
    ログインテスト
    """

    url = "/api/auth/jwt/create/"

    def setUp(self) -> None:
        self.user = get_user_model().objects.create_user("john", "lennon@thebeatles.com", "johnpassword")

        # パスワードをpasswordに
        users = get_user_model().objects.all()
        for user in users:
            user.set_password("password")
            user.save()

    def test_post(self):
        payload = {"username": self.user.username, "password": "wrongpassword"}
        response = self.client.post(self.url, payload)
        self.assertEqual(response.status_code, 401)

        payload = {"username": self.user.username, "password": "password"}

        response = self.client.post(self.url, payload)
        self.assertEqual(response.status_code, 200)

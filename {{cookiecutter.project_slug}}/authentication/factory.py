from django.contrib.auth import get_user_model
from django.utils import timezone

from factory.django import DjangoModelFactory
from factory import Faker, Sequence


class AuthUserFactory(DjangoModelFactory):
    """
    認証ユーザーのモッククラス
    """

    class Meta:
        model = get_user_model()

    password = Faker("password")
    first_name = Faker("first_name", locale="ja_jp")
    last_name = Faker("last_name", locale="ja_jp")
    username = Sequence(lambda n: "user%d" % n)
    email = Faker("email")
    last_login = Faker("date_time", tzinfo=timezone.get_current_timezone())
    date_joined = Faker("date_time", tzinfo=timezone.get_current_timezone())
    is_superuser = 0
    is_staff = 0
    is_active = 1

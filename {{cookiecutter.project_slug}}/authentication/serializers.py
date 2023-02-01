from django.contrib.auth.models import update_last_login
from rest_framework.exceptions import AuthenticationFailed
from rest_framework_simplejwt.serializers import TokenObtainPairSerializer


class LoginTokenObtainSerializer(TokenObtainPairSerializer):
    def validate(self, attrs):
        data = super(LoginTokenObtainSerializer, self).validate(attrs)
        refresh = self.get_token(self.user)

        data["refresh"] = str(refresh)
        data["access"] = str(refresh.access_token)

        update_last_login(None, self.user)

        return data

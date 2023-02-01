from django.conf import settings
from django.contrib.auth import authenticate, login, logout
from django.contrib.auth.views import (
    PasswordResetCompleteView,
    PasswordResetConfirmView,
    PasswordResetDoneView,
    PasswordResetView,
)
from django.urls import reverse_lazy
from django.utils import timezone
from drf_spectacular.types import OpenApiTypes
from drf_spectacular.utils import OpenApiParameter, OpenApiResponse, extend_schema, extend_schema_view
from rest_framework import status, serializers
from rest_framework.permissions import IsAuthenticated
from rest_framework.response import Response
from rest_framework.views import APIView
from rest_framework_simplejwt import views as jwt_views
from rest_framework_simplejwt.tokens import RefreshToken
from djoser.social.views import ProviderAuthView

from .serializers import LoginTokenObtainSerializer

class LoginAPIView(jwt_views.TokenObtainPairView):
    """
    ログインAPI
    """
    serializer_class = LoginTokenObtainSerializer

    def post(self, request, *args, **kwargs):
        return super().post(request, *args, **kwargs)


class CustomProviderAuthView(ProviderAuthView):
    """
    OAuthログインAPI
    """
    @extend_schema(
        parameters=[
            OpenApiParameter("redirect_uri", OpenApiTypes.UUID, OpenApiParameter.QUERY, description="リダイレクトURI")
        ]
    )
    def get(self, request, *args, **kwargs):
        return super().get(request, *args, **kwargs)


class LogoutAPIView(APIView):
    """
    ログアウトAPI
    """
    permission_classes = (IsAuthenticated,)

    class RefreshTokenSerializer(serializers.Serializer):
        refresh_token = serializers.CharField(help_text="リフレッシュトークン")

    @extend_schema(
        request=RefreshTokenSerializer,
        responses={
            200: OpenApiResponse(
                description="OK",
            ),
            400: OpenApiResponse(
                description="Bad Request",
            ),
        },
    )
    def post(self, request, *args, **kwargs):
        refresh_token = request.data.get("refresh_token")
        token = RefreshToken(token=refresh_token)
        token.blacklist()
        return Response(status=status.HTTP_200_OK)


class PasswordReset(PasswordResetView):
    """
    パスワード再設定ページ
    """

    from_email = settings.DEFAULT_FROM_EMAIL
    success_url = reverse_lazy("auth:password_reset_done")


class PasswordResetDone(PasswordResetDoneView):
    """
    パスワード再設定確認ページ
    """
    pass


class PasswordResetConfirm(PasswordResetConfirmView):
    """
    新規パスワード入力ページ
    """
    pass


class PasswordResetComplete(PasswordResetCompleteView):
    """
    パスワード変更完了ページ
    """
    pass

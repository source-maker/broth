from dj_rest_auth import views as dj_rest_auth_views
from django.urls import path, include
from rest_framework_simplejwt import views as jwt_views

from authentication import views

app_name = "auth-api"
urlpatterns = [
    path("login/", views.LoginView.as_view(), name="login"),
    path("logout/", dj_rest_auth_views.LogoutView.as_view(), name="logout"),
    path("password/change/", dj_rest_auth_views.PasswordChangeView.as_view(), name="password-change"),
    path("password/reset/", dj_rest_auth_views.PasswordResetView.as_view(), name="password-reset"),
    path(
        "password/reset/confirm/", dj_rest_auth_views.PasswordResetConfirmView.as_view(), name="password-reset-confirm"
    ),
    path("token/refresh/", jwt_views.TokenRefreshView.as_view(), name="token-refresh"),
    path("token/verify/", jwt_views.TokenVerifyView.as_view(), name="token-verify"),
]

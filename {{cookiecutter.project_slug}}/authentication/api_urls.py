from django.urls import path, re_path
from rest_framework_simplejwt import views as jwt_views

from authentication import views

app_name = "auth-api"
urlpatterns = [
    re_path(r"^auth/jwt/create/?", views.LoginAPIView.as_view(), name="jwt-create"),
    re_path(r"^auth/jwt/refresh/?", jwt_views.TokenRefreshView.as_view(), name="jwt-refresh"),
    re_path(r"^auth/jwt/verify/?", jwt_views.TokenVerifyView.as_view(), name="jwt-verify"),
    re_path(
        r"^auth/o/(?P<provider>\S+)/$",
        views.CustomProviderAuthView.as_view(),
        name="provider-auth",
    ),
]

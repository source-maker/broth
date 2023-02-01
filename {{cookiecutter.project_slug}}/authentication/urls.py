from django.urls import path
from authentication import views

app_name = "auth"
urlpatterns = [
    path("account/password_reset/", views.PasswordReset.as_view(), name="password_reset"),
    path("account/password_reset/done/", views.PasswordResetDone.as_view(), name="password_reset_done"),
    path(
        "account/password_reset/reset/<uidb64>/<token>/",
        views.PasswordResetConfirm.as_view(),
        name="password_reset_confirm",
    ),
    path("account/password_change/reset/done/", views.PasswordResetComplete.as_view(), name="password_reset_complete"),
]

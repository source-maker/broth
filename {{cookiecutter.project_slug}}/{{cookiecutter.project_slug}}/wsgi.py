"""
WSGI config 
It exposes the WSGI callable as a module-level variable named ``application``.

For more information on this file, see
https://docs.djangoproject.com/en/3.2/howto/deployment/wsgi/
"""

import os

from django.core.wsgi import get_wsgi_application

ENV_VAR = os.getenv("django-env", "local")
SETTING = "{{cookiecutter.project_slug}}.settings.{}".format(ENV_VAR)

os.environ.setdefault("DJANGO_SETTINGS_MODULE", SETTING)

application = get_wsgi_application()
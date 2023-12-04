#!/bin/bash
python manage.py collectstatic --noinput
python manage.py makemigrations
python manage.py migrate
gunicorn {{cookiecutter.project_core_name}}.wsgi:application --bind 0.0.0.0:8000

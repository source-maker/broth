FROM python:{{ cookiecutter.python_version }}

ENV LC_ALL=C.UTF-8 LANG=C.UTF-8
ENV PYTHONUNBUFFERED=1
ENV PYTHONDONTWRITEBYTECODE 1
ENV DEBIAN_FRONTEND noninteractive

# 明示しておく
ENTRYPOINT ["/bin/bash", "-c"]

WORKDIR /{{cookiecutter.project_slug}}

RUN apt-get -y update && apt-get -y upgrade
RUN apt-get install -y curl gettext postgresql-client dos2unix zsh

ADD . /{{cookiecutter.project_slug}}
RUN pip install --upgrade pip && pip install pipenv

#　pipfile.lockを作成するために初回起動時のみ使用すること
RUN pipenv lock --dev

RUN pipenv sync --system --dev

# コンテナ軽量化
RUN rm -rf /var/lib/apt/lists/*

# CRLF対策
COPY docker/django/entrypoint.sh /entrypoint.sh
RUN dos2unix /entrypoint.sh
RUN chmod 777 /entrypoint.sh
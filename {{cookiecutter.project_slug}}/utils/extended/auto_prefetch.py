"""
https://github.com/GeeWee/django-auto-prefetching
"""
from typing import Type

from rest_framework.serializers import ModelSerializer

from django.core.exceptions import FieldError
from django_auto_prefetching import _prefetch


def prefetch(queryset, serializer: Type[ModelSerializer], include_select=False):
    """Prefetchが必要な関連テーブルを計算"""
    select_related, prefetch_related = _prefetch(serializer)
    try:
        if select_related and include_select:
            queryset = queryset.select_related(*select_related)
        if prefetch_related:
            queryset = queryset.prefetch_related(*prefetch_related)
        return queryset
    except FieldError as error:
        raise ValueError(
            f"Calculated wrong field in select_related.\
            Do you have a nested serializer for a ForeignKey where"
            f"you've forgotten to specify many=True? Original error: {error}"
        ) from error

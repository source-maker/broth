from django.conf import settings
from storages.backends.s3boto3 import S3Boto3Storage


class PrivateStorage(S3Boto3Storage):
    """PrivateStorage File Class"""

    bucket_name = settings.MEDIA_BUCKET_NAME
    location = ""
    file_overwrite = True
    default_acl = "private"
    bucket_acl = "private"
    encryption = True
    custom_domain = False
    querystring_expire = 1800  # Expires in 30 minutes (1800 seconds)
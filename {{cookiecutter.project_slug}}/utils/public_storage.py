from django.conf import settings
from storages.backends.s3boto3 import S3Boto3Storage


class ImageStorage(S3Boto3Storage):
    """PublicFile Storage Class"""

    bucket_name = settings.STATIC_BUCKET_NAME
    location = ""
    file_overwrite = False
    default_acl = "public-read"
    encryption = True

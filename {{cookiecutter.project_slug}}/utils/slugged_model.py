from django.db import models
from django.utils.crypto import get_random_string


class SluggedModel(models.Model):
    slug_length = 12
    slug = models.SlugField(max_length=25, unique=True, null=True, blank=True)

    ALLOWED_CHARS = "abcdefghijklmnopqrstuvwxyz0123456789"

    class Meta:
        abstract = True

    def save(self, *args, **kwargs):
        if not self.slug:
            self.slug = self.generate_random_slug()
        super().save(*args, **kwargs)

    def generate_random_slug(self):
        for i in range(3):
            slug = get_random_string(self.slug_length, allowed_chars=self.ALLOWED_CHARS)
            if not self._meta.model.objects.filter(slug=slug).exists():
                return slug
        raise Exception("slug calculation failed")

from django.conf import settings
from django.core.mail import EmailMultiAlternatives
from django.template import loader


class EmailSenderClient:
    """
    Email client emails.
    """
    def __init__(self, from_email=None):
        self.from_email = from_email if from_email else settings.DEFAULT_FROM_EMAIL

    def send_mail(
        self,
        context,
        to_email,
        subject_template_name,
        email_template_name,
        reply_to=None,
        attachments=None,
    ):
        """
        Send a django.core.mail.EmailMultiAlternatives to `to_email`.
        """
        subject = loader.render_to_string(subject_template_name, context)
        # Email subject *must not* contain newlines
        subject = "".join(subject.splitlines())
        body = loader.render_to_string(email_template_name, context)

        if type(to_email) is str:
            to_email = [to_email]
        if type(reply_to) is str:
            reply_to = [reply_to]

        email_message = EmailMultiAlternatives(subject, body, self.from_email, to_email, reply_to=reply_to)
        if attachments:
            for attachment in attachments:
                email_message.attach(attachment.name, attachment.read())
        email_message.send()

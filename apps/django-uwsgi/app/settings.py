import os

SECRET_KEY = "benchmark-only-not-for-production"
DEBUG = False
ALLOWED_HOSTS = ["*"]

ROOT_URLCONF = "app.urls"
WSGI_APPLICATION = "app.wsgi.application"

INSTALLED_APPS = [
    "django.contrib.contenttypes",
    "django.contrib.auth",
]

MIDDLEWARE = []

DATABASES = {}

USE_TZ = True

# The endpoint the blocking view calls to simulate the slow 3rd-party API.
THIRD_PARTY_URL = os.environ.get("THIRD_PARTY_URL", "http://localhost:9000/delay")

# Generous — must exceed the mock's MAX_MS so we measure the app's own
# behavior under blocking I/O, not an artificial client-side timeout.
THIRD_PARTY_TIMEOUT_S = float(os.environ.get("THIRD_PARTY_TIMEOUT_S", "65"))

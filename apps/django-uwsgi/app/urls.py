from django.urls import path

from . import views

urlpatterns = [
    path("call", views.blocking_view),
    path("health", views.health),
    path("metrics", views.metrics),
]

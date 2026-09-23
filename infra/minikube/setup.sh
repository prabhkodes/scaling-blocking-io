#!/usr/bin/env bash
# Dedicated minikube profile for this project — kept separate from any other
# local clusters (this machine had a pre-existing "minikube" default profile
# running unrelated work, so this one gets its own name/context).
set -euo pipefail

PROFILE="scaling-blocking-io"

minikube start -p "$PROFILE" --driver=docker --cpus=3 --memory=2800mb
minikube -p "$PROFILE" addons enable metrics-server

kubectl config use-context "$PROFILE"
kubectl get nodes

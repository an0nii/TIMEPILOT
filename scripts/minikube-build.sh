#!/bin/sh
set -eu

IMAGE="${IMAGE:-timepilot:k8s}"
VERSION="${VERSION:-k8s-local}"
IMAGE_ARCHIVE="$(mktemp /tmp/timepilot-image.XXXXXX.tar)"

cleanup() {
  rm -f "${IMAGE_ARCHIVE}"
}
trap cleanup EXIT INT TERM

eval "$(minikube docker-env)"
if ! docker build --build-arg "VERSION=${VERSION}" -t "${IMAGE}" .; then
  echo "In-cluster build failed; retrying with the host Docker daemon." >&2
  eval "$(minikube docker-env --unset)"
  docker build --build-arg "VERSION=${VERSION}" -t "${IMAGE}" .
fi

# Loading an archive is deterministic both for the minikube and host daemons.
docker image save -o "${IMAGE_ARCHIVE}" "${IMAGE}"
minikube image load "${IMAGE_ARCHIVE}"

echo "Image ${IMAGE} is available in minikube."

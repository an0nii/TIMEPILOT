.PHONY: test build run up down logs migrate minikube-build k8s-deploy k8s-status k8s-scale k8s-url k8s-delete logging-deploy logging-status logging-ui logging-check logging-delete

test:
	go test ./...

build:
	go build -o timepilot ./cmd/timepilot

run:
	go run ./cmd/timepilot server

up:
	docker compose up --build -d

down:
	docker compose down

logs:
	docker compose logs -f app

migrate:
	go run ./cmd/timepilot migrate

minikube-build:
	./scripts/minikube-build.sh

k8s-deploy:
	./scripts/minikube-deploy.sh

k8s-status:
	kubectl get pods,services,statefulsets,deployments,jobs,pvc -n timepilot -o wide

k8s-scale:
	kubectl scale deployment/timepilot --replicas=3 -n timepilot
	kubectl rollout status deployment/timepilot -n timepilot --timeout=180s

k8s-url:
	minikube service timepilot -n timepilot --url

k8s-delete:
	kubectl delete namespace timepilot --ignore-not-found

logging-deploy:
	./observability/deploy.sh

logging-status:
	kubectl get pods,services,pvc -n observability -o wide

logging-ui:
	kubectl port-forward -n observability service/victoria-logs 9428:9428

logging-check:
	./observability/check.sh

logging-delete:
	kubectl delete namespace observability --ignore-not-found

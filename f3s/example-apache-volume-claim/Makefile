apply:
	kubectl apply -f apache-persistent-volume.yaml
	kubectl apply -f apache-service.yaml
	kubectl apply -f apache-deployment.yaml
	kubectl apply -f apache-ingress.yaml

delete:
	kubectl delete -f apache-ingress.yaml
	kubectl delete -f apache-service.yaml
	kubectl delete -f apache-deployment.yaml
	kubectl delete -f apache-persistent-volume.yaml

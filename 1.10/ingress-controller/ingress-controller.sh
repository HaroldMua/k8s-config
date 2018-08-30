#!/bin/bash
# 00-namespace.yaml  01-default-backend.yaml  02-configmap.yaml  03-tcp-services-configmap.yaml  04-udp-services-configmap.yaml  50-rbac.yaml  51-with-rbac.yaml
case $1 in
	up)
		kubectl apply -f 00-namespace.yaml
		kubectl apply -f 01-default-backend.yaml
		kubectl apply -f 02-configmap.yaml
		kubectl apply -f 03-tcp-services-configmap.yaml
		kubectl apply -f 04-udp-services-configmap.yaml
		kubectl apply -f 50-rbac.yaml
		kubectl apply -f 51-with-rbac.yaml
		;;
	down)
		kubectl delete -f 51-with-rbac.yaml
		kubectl delete -f 50-rbac.yaml
		kubectl delete -f 04-udp-services-configmap.yaml
		kubectl delete -f 03-tcp-services-configmap.yaml
		kubectl delete -f 02-configmap.yaml
		kubectl delete -f 01-default-backend.yaml
		kubectl delete -f 00-namespace.yaml
		;;

	*)
		echo "Usage: ./ingress-controller.sh [up|down]"
		;;
esac


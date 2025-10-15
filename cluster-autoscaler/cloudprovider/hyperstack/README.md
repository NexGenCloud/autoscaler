# Cluster Autoscaler for Hyperstack

The cluster autoscaler for Hyperstack scales worker nodes within any specified Hyperstack Kubernetes cluster's node group. It runs as a Deployment on a worker node in the cluster. This README will go over some of the necessary steps required to get the cluster autoscaler up and running.

# Configuration

## Setup configmap

The autoscaler deployment requires configmap named `cluster-autoscaler-cm` in `kube-system` namespace. Create a `.env` file with the folowing values
```
HYPERSTACK_API_KEY=
HYPERSTACK_API_SERVER=
```
Value of `HYPERSTACK_API_SERVER` is optional. Default value is `https://infrahub-api.nexgencloud.com/v1`.
Create the configmap with this env file-
```
kubectl -n kube-system create cm cluster-autoscaler-cm --from-env-file=/path/to/.env
```

## Setup autoscaler

Install cluster autoscaler with the command below, and make sure the workload are running.
```
kubectl apply -f https://raw.githubusercontent.com/NexGenCloud/autoscaler/refs/heads/master/cluster-autoscaler/cloudprovider/hyperstack/example/deployment.yaml
```

## Behavior

Parameters of the autoscaler (the minimum/maximum values) are configured through the Hyperstack API and subsequently reflected by the node group objects. The autoscaler periodically picks up the configuration from the API and adjusts the behavior accordingly. The autoscaler operates only when maximum > minimum. By default, the autoscaler refreshes every 60 seconds.
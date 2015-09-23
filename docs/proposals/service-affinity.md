# Proposal: Affinity Priority for pods of different RC/services

When deploying a multi-layer application, a typical model is to spread the pods of same layer while placing pods of different layers together.
Currently the scheduler can spread pods from the same RC and/or Service, but cannot affiliate a pod to another.

This proposal is to support the affinity priority between pods of different RC/services.

As a first step, we use label selector to implement. **[see code here](https://github.com/kubernetes/kubernetes/compare/master...kevin-wangzefeng:service-sort-affinity)**


**Here is a example of usage**
The definition of Redis pod:
```
apiVersion: v1
kind: Pod
metadata:
  name: redis
  labels:
    service: redis
spec:
  containers:
  - name: redis
    image: redis
```

The definition of app pod that accesses Redis:
```
apiVersion: v1
kind: Pod
metadata:
  name: nodejs
spec:
  containers:
  - name: nodejs
    image: nodejs
  affinitySelector:
    service: redis
```

Benefits:
    Reduce the forward times of requests from application to RC/services it accesses, to gain shorter delay and light network load.

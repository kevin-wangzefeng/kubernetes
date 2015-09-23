
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

The definition of application pod that accesses Redis:
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
* Reduce the forward times of requests from application to RC/services it accesses, to gain shorter delay and light network load.

> ### 限制与当前遇到的问题
> 与nodeselector类似，需要新增label selector，但语义比较局限。跟踪社区#341和#367动态。  
> 受限于label selector的语义问题（不支持集合），当前对多个服务做亲和需要使用不同的 label key
> 多种scheduler算法的结果出现冲突（逻辑上的）时，如何权衡。比如当被亲和的pod数量少于要亲和的pod数量时，spread和affinity的结果相矛盾，仅仅依靠权重配置效果不好。

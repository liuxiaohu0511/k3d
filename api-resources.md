# k3d 集群内置资源类型（`kubectl api-resources` 输出）

## 核心组 `v1`

| NAME | SHORTNAMES | APIVERSION | NAMESPACED | KIND |
|---|---|---|---|---|
| bindings | | v1 | true | Binding |
| componentstatuses | cs | v1 | false | ComponentStatus |
| configmaps | cm | v1 | true | ConfigMap |
| endpoints | ep | v1 | true | Endpoints |
| events | ev | v1 | true | Event |
| limitranges | limits | v1 | true | LimitRange |
| namespaces | ns | v1 | false | Namespace |
| nodes | no | v1 | false | Node |
| persistentvolumeclaims | pvc | v1 | true | PersistentVolumeClaim |
| persistentvolumes | pv | v1 | false | PersistentVolume |
| pods | po | v1 | true | Pod |
| podtemplates | | v1 | true | PodTemplate |
| replicationcontrollers | rc | v1 | true | ReplicationController |
| resourcequotas | quota | v1 | true | ResourceQuota |
| secrets | | v1 | true | Secret |
| serviceaccounts | sa | v1 | true | ServiceAccount |
| services | svc | v1 | true | Service |

## 准入控制 `admissionregistration.k8s.io/v1`

| NAME | SHORTNAMES | APIVERSION | NAMESPACED | KIND |
|---|---|---|---|---|
| mutatingwebhookconfigurations | | admissionregistration.k8s.io/v1 | false | MutatingWebhookConfiguration |
| validatingwebhookconfigurations | | admissionregistration.k8s.io/v1 | false | ValidatingWebhookConfiguration |

## API 扩展

| NAME | SHORTNAMES | APIVERSION | NAMESPACED | KIND |
|---|---|---|---|---|
| customresourcedefinitions | crd, crds | apiextensions.k8s.io/v1 | false | CustomResourceDefinition |
| apiservices | | apiregistration.k8s.io/v1 | false | APIService |

## 应用管理 `apps/v1`

| NAME | SHORTNAMES | APIVERSION | NAMESPACED | KIND |
|---|---|---|---|---|
| controllerrevisions | | apps/v1 | true | ControllerRevision |
| daemonsets | ds | apps/v1 | true | DaemonSet |
| deployments | deploy | apps/v1 | true | Deployment |
| replicasets | rs | apps/v1 | true | ReplicaSet |
| statefulsets | sts | apps/v1 | true | StatefulSet |

## 认证与授权

| NAME | SHORTNAMES | APIVERSION | NAMESPACED | KIND |
|---|---|---|---|---|
| tokenreviews | | authentication.k8s.io/v1 | false | TokenReview |
| localsubjectaccessreviews | | authorization.k8s.io/v1 | true | LocalSubjectAccessReview |
| selfsubjectaccessreviews | | authorization.k8s.io/v1 | false | SelfSubjectAccessReview |
| selfsubjectrulesreviews | | authorization.k8s.io/v1 | false | SelfSubjectRulesReview |
| subjectaccessreviews | | authorization.k8s.io/v1 | false | SubjectAccessReview |

## 自动扩缩容 / 任务调度

| NAME | SHORTNAMES | APIVERSION | NAMESPACED | KIND |
|---|---|---|---|---|
| horizontalpodautoscalers | hpa | autoscaling/v2 | true | HorizontalPodAutoscaler |
| cronjobs | cj | batch/v1 | true | CronJob |
| jobs | | batch/v1 | true | Job |

## 证书 / 协调 / 服务发现

| NAME | SHORTNAMES | APIVERSION | NAMESPACED | KIND |
|---|---|---|---|---|
| certificatesigningrequests | csr | certificates.k8s.io/v1 | false | CertificateSigningRequest |
| leases | | coordination.k8s.io/v1 | true | Lease |
| endpointslices | | discovery.k8s.io/v1 | true | EndpointSlice |
| events | ev | events.k8s.io/v1 | true | Event |

## 流控

| NAME | SHORTNAMES | APIVERSION | NAMESPACED | KIND |
|---|---|---|---|---|
| flowschemas | | flowcontrol.apiserver.k8s.io/v1beta3 | false | FlowSchema |
| prioritylevelconfigurations | | flowcontrol.apiserver.k8s.io/v1beta3 | false | PriorityLevelConfiguration |

## k3s / Helm 专属（k3d集群特有）

| NAME | SHORTNAMES | APIVERSION | NAMESPACED | KIND |
|---|---|---|---|---|
| helmchartconfigs | | helm.cattle.io/v1 | true | HelmChartConfig |
| helmcharts | | helm.cattle.io/v1 | true | HelmChart |
| addons | | k3s.cattle.io/v1 | true | Addon |

## 监控指标

| NAME | SHORTNAMES | APIVERSION | NAMESPACED | KIND |
|---|---|---|---|---|
| nodes | | metrics.k8s.io/v1beta1 | false | NodeMetrics |
| pods | | metrics.k8s.io/v1beta1 | true | PodMetrics |

## 网络 `networking.k8s.io/v1`

| NAME | SHORTNAMES | APIVERSION | NAMESPACED | KIND |
|---|---|---|---|---|
| ingressclasses | | networking.k8s.io/v1 | false | IngressClass |
| ingresses | ing | networking.k8s.io/v1 | true | Ingress |
| networkpolicies | netpol | networking.k8s.io/v1 | true | NetworkPolicy |

## 节点 / 调度 / 中断预算

| NAME | SHORTNAMES | APIVERSION | NAMESPACED | KIND |
|---|---|---|---|---|
| runtimeclasses | | node.k8s.io/v1 | false | RuntimeClass |
| poddisruptionbudgets | pdb | policy/v1 | true | PodDisruptionBudget |
| priorityclasses | pc | scheduling.k8s.io/v1 | false | PriorityClass |

## 权限控制 `rbac.authorization.k8s.io/v1`

| NAME | SHORTNAMES | APIVERSION | NAMESPACED | KIND |
|---|---|---|---|---|
| clusterrolebindings | | rbac.authorization.k8s.io/v1 | false | ClusterRoleBinding |
| clusterroles | | rbac.authorization.k8s.io/v1 | false | ClusterRole |
| rolebindings | | rbac.authorization.k8s.io/v1 | true | RoleBinding |
| roles | | rbac.authorization.k8s.io/v1 | true | Role |

## 存储 `storage.k8s.io/v1`

| NAME | SHORTNAMES | APIVERSION | NAMESPACED | KIND |
|---|---|---|---|---|
| csidrivers | | storage.k8s.io/v1 | false | CSIDriver |
| csinodes | | storage.k8s.io/v1 | false | CSINode |
| csistoragecapacities | | storage.k8s.io/v1 | true | CSIStorageCapacity |
| storageclasses | sc | storage.k8s.io/v1 | false | StorageClass |
| volumeattachments | | storage.k8s.io/v1 | false | VolumeAttachment |

## Traefik CRD（`traefik.containo.us/v1alpha1`）

> 🎯 这就是你之前 `Middleware` 报错的答案——你集群里实际的 apiVersion 是 `traefik.containo.us/v1alpha1`，不是 `traefik.io/v1alpha1`。之前推测对了。

| NAME | SHORTNAMES | APIVERSION | NAMESPACED | KIND |
|---|---|---|---|---|
| ingressroutes | | traefik.containo.us/v1alpha1 | true | IngressRoute |
| ingressroutetcps | | traefik.containo.us/v1alpha1 | true | IngressRouteTCP |
| ingressrouteudps | | traefik.containo.us/v1alpha1 | true | IngressRouteUDP |
| middlewares | | traefik.containo.us/v1alpha1 | true | Middleware |
| middlewaretcps | | traefik.containo.us/v1alpha1 | true | MiddlewareTCP |
| serverstransports | | traefik.containo.us/v1alpha1 | true | ServersTransport |
| tlsoptions | | traefik.containo.us/v1alpha1 | true | TLSOption |
| tlsstores | | traefik.containo.us/v1alpha1 | true | TLSStore |
| traefikservices | | traefik.containo.us/v1alpha1 | true | TraefikService |

---

## 顺手回到之前的问题

现在你的 `Middleware` YAML 应该改成：

```yaml
apiVersion: traefik.containo.us/v1alpha1   # 不是 traefik.io！你的集群实测是这个
kind: Middleware
metadata:
  name: nginx-stripprefix
spec:
  stripPrefix:
    prefixes:
      - /nginx
```

这份清单可以直接说明，你的 k3d 集群里 Traefik 用的是**旧版 CRD 分组名**（`traefik.containo.us`），对应的是 Traefik v2.x。如果以后想装 Traefik v3（走 `traefik.io` 分组），需要单独升级 Traefik 组件，k3s 默认捆绑的版本不会自动跟着变。
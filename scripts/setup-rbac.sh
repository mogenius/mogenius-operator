#!/bin/bash

kubectx

kubectl apply -f -<<EOF
apiVersion: v1
kind: ServiceAccount
metadata:
   name: mogenius-k8s-manager-service-account
   namespace: default
EOF

kubectl apply -f -<<EOF
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: mogenius-k8s-manager-cluster-role
rules:
  - apiGroups: ["", "extensions", "apps"]
    resources: ["pods", "services", "endpoints"]
    verbs: ["list", "get", "watch"]
EOF

kubectl apply -f -<<EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: mogenius-k8s-manager-cluster-role-binding
subjects:
  - kind: ServiceAccount
    name: mogenius-k8s-manager-service-account
    apiGroup: ""
    namespace: default
roleRef:
  kind: ClusterRole
  name: mogenius-k8s-manager-cluster-role
  apiGroup: rbac.authorization.k8s.io
EOF
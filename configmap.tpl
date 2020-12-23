apiVersion: v1
kind: ConfigMap
metadata:
  name: demo-configmap
  namespace: {{ .Namespace }}
data:
  Name: demo
{{- define "flux-terragrunt-controller.fullname" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "flux-terragrunt-controller.labels" -}}
app.kubernetes.io/name: {{ .Chart.Name }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "flux-terragrunt-controller.selectorLabels" -}}
app.kubernetes.io/name: {{ .Chart.Name }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{/*
Get the service account name for the controller and runner.
Prefers runner.serviceAccount.name if set, falls back to serviceAccount.name,
then to the chart fullname.
*/}}
{{- define "flux-terragrunt-controller.serviceAccountName" -}}
{{- if .Values.runner.serviceAccount.name }}
{{- .Values.runner.serviceAccount.name }}
{{- else if .Values.serviceAccount.name }}
{{- .Values.serviceAccount.name }}
{{- else }}
{{- include "flux-terragrunt-controller.fullname" . }}
{{- end }}
{{- end -}}

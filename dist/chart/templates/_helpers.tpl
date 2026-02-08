{{- define "chart.name" -}}
{{- if .Chart }}
  {{- if .Chart.Name }}
    {{- .Chart.Name | trunc 63 | trimSuffix "-" }}
  {{- else if .Values.nameOverride }}
    {{ .Values.nameOverride | trunc 63 | trimSuffix "-" }}
  {{- else }}
    sleepod-controller
  {{- end }}
{{- else }}
  sleepod-controller
{{- end }}
{{- end }}


{{- define "chart.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}


{{- define "chart.labels" -}}
{{- if .Chart.AppVersion -}}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
{{- if .Chart.Version }}
helm.sh/chart: {{ .Chart.Version | quote }}
{{- end }}
app.kubernetes.io/name: {{ include "chart.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}


{{- define "chart.selectorLabels" -}}
app.kubernetes.io/name: {{ include "chart.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}


{{- define "chart.hasMutatingWebhooks" -}}
{{- $hasMutating := false }}
{{- range . }}
  {{- if eq .type "mutating" }}
    $hasMutating = true }}{{- end }}
{{- end }}
{{ $hasMutating }}}}{{- end }}


{{- define "chart.hasValidatingWebhooks" -}}
{{- $hasValidating := false }}
{{- range . }}
  {{- if eq .type "validating" }}
    $hasValidating = true }}{{- end }}
{{- end }}
{{ $hasValidating }}}}{{- end }}

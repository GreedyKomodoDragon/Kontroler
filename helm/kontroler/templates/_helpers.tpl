{{/*
Expand the name of the chart.
*/}}
{{- define "kontroler.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "kontroler.fullname" -}}
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

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "kontroler.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "kontroler.labels" -}}
helm.sh/chart: {{ include "kontroler.chart" . }}
{{ include "kontroler.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "kontroler.selectorLabels" -}}
app.kubernetes.io/name: {{ include "kontroler.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "kontroler.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "kontroler.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "kontroler.server.selectorLabels" -}}
app.kubernetes.io/name: {{ .Values.server.name }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Common Server labels
*/}}
{{- define "kontroler.server.labels" -}}
helm.sh/chart: {{ include "kontroler.chart" . }}
{{ include "kontroler.server.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "kontroler.ui.selectorLabels" -}}
app.kubernetes.io/name: {{ .Values.ui.name }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Common Server labels
*/}}
{{- define "kontroler.ui.labels" -}}
helm.sh/chart: {{ include "kontroler.chart" . }}
{{ include "kontroler.ui.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}
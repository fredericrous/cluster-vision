{{/*
Expand the name of the chart.
*/}}
{{- define "cluster-vision.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "cluster-vision.fullname" -}}
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
{{- define "cluster-vision.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels.
*/}}
{{- define "cluster-vision.labels" -}}
helm.sh/chart: {{ include "cluster-vision.chart" . }}
{{ include "cluster-vision.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels.
*/}}
{{- define "cluster-vision.selectorLabels" -}}
app.kubernetes.io/name: {{ include "cluster-vision.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use.
*/}}
{{- define "cluster-vision.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "cluster-vision.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Build DATA_SOURCES JSON from dataSources values with computed mount paths.
*/}}
{{- define "cluster-vision.dataSourcesJSON" -}}
{{- $sources := list -}}
{{- range $i, $ds := .Values.dataSources -}}
{{- $source := dict "name" $ds.name "type" $ds.type "path" (printf "/data/source-%d/data" $i) -}}
{{- $sources = append $sources $source -}}
{{- end -}}
{{- $sources | toJson -}}
{{- end -}}

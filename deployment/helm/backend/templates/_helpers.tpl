{{- define "backend.name" -}}
backend
{{- end -}}

{{- define "backend.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name .Chart.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{- define "backend.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version }}
app.kubernetes.io/name: {{ include "backend.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "backend.selectorLabels" -}}
app.kubernetes.io/name: {{ include "backend.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "backend.cpRole" -}}
{{- if eq .Values.mode "split" -}}cp{{- else -}}combined{{- end -}}
{{- end -}}

{{- define "backend.dpRole" -}}
{{- if eq .Values.mode "split" -}}dp{{- else -}}combined{{- end -}}
{{- end -}}

{{- define "backend.validate" -}}
{{- if not (has .Values.mode (list "combined" "split")) -}}
{{- fail "backend.mode must be combined or split" -}}
{{- end -}}
{{- end -}}

{{- define "backend.envFrom" -}}
- configMapRef:
    name: {{ include "backend.fullname" . }}
{{- if .Values.existingSecret }}
- secretRef:
    name: {{ .Values.existingSecret }}
{{- end }}
{{- end -}}

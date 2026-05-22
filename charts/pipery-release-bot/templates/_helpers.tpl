{{- define "pipery-release-bot.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "pipery-release-bot.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{- define "pipery-release-bot.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | quote }}
app.kubernetes.io/name: {{ include "pipery-release-bot.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "pipery-release-bot.selectorLabels" -}}
app.kubernetes.io/name: {{ include "pipery-release-bot.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "pipery-release-bot.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "pipery-release-bot.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{- define "pipery-release-bot.configName" -}}
{{- default (printf "%s-config" (include "pipery-release-bot.fullname" .)) .Values.config.existingConfigMap -}}
{{- end -}}

{{- define "pipery-release-bot.privateKeySecretName" -}}
{{- default (printf "%s-private-key" (include "pipery-release-bot.fullname" .)) .Values.privateKey.existingSecret -}}
{{- end -}}

{{- define "pipery-release-bot.apiTokenSecretName" -}}
{{- default (printf "%s-api-token" (include "pipery-release-bot.fullname" .)) .Values.apiToken.existingSecret -}}
{{- end -}}

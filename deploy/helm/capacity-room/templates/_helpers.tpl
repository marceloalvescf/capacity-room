{{/* The base name of every object: the release name, cut so that a suffix
fits a DNS label. */}}
{{- define "capacity-room.fullname" -}}
{{- .Release.Name | trunc 50 | trimSuffix "-" -}}
{{- end -}}

{{- define "capacity-room.labels" -}}
app.kubernetes.io/name: capacity-room
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version }}
{{- end -}}

{{- define "capacity-room.selectorLabels" -}}
app.kubernetes.io/name: capacity-room
app.kubernetes.io/instance: {{ .root.Release.Name }}
app.kubernetes.io/component: {{ .component }}
{{- end -}}

{{- define "capacity-room.image" -}}
{{- printf "%s:%s" .Values.image.repository (.Values.image.tag | default .Chart.AppVersion) -}}
{{- end -}}

{{- define "capacity-room.podSecurityContext" -}}
runAsNonRoot: true
runAsUser: 10001
runAsGroup: 10001
fsGroup: 10001
seccompProfile:
  type: RuntimeDefault
{{- end -}}

{{- define "capacity-room.containerSecurityContext" -}}
runAsNonRoot: true
readOnlyRootFilesystem: true
allowPrivilegeEscalation: false
capabilities:
  drop: ["ALL"]
{{- end -}}

{{- define "capacity-room.databaseEnv" -}}
- name: CAPACITYROOM_DATABASE_DSN
  valueFrom:
    secretKeyRef:
      name: {{ .Values.database.existingSecret }}
      key: {{ .Values.database.key }}
{{- end -}}

{{/* platform.toml: the chart's three paths, then each key of
.Values.platform. The schema allows an integer or a list of strings, and an
integer is written as one, since Helm reads numbers as floats. */}}
{{- define "capacity-room.platformToml" -}}
server_secret_path = {{ printf "/opt/capacity-room/secret/%s" .Values.serverSecret.key | toJson }}
credential_dir = "/opt/capacity-room/credentials"
ca_bundle_dir = "/opt/capacity-room/ca"
{{- range $key, $value := .Values.platform }}
{{- if kindIs "slice" $value }}
{{ $key }} = {{ toJson $value }}
{{- else }}
{{ $key }} = {{ int64 $value }}
{{- end }}
{{- end }}
{{- end -}}

{{- define "capacity-room.volumes" -}}
- name: tmp
  emptyDir: {}
- name: settings
  configMap:
    name: {{ include "capacity-room.fullname" . }}
- name: server-secret
  secret:
    secretName: {{ .Values.serverSecret.existingSecret }}
    defaultMode: 0440
    items:
      - key: {{ .Values.serverSecret.key }}
        path: {{ .Values.serverSecret.key }}
{{- with .Values.credentials.existingSecret }}
- name: credentials
  secret:
    secretName: {{ . }}
    defaultMode: 0440
{{- end }}
{{- with .Values.caBundles.existingConfigMap }}
- name: ca
  configMap:
    name: {{ . }}
{{- end }}
{{- end -}}

{{- define "capacity-room.volumeMounts" -}}
- name: tmp
  mountPath: /tmp
- name: settings
  mountPath: /opt/capacity-room/platform.toml
  subPath: platform.toml
  readOnly: true
- name: server-secret
  mountPath: /opt/capacity-room/secret
  readOnly: true
{{- if .Values.credentials.existingSecret }}
- name: credentials
  mountPath: /opt/capacity-room/credentials
  readOnly: true
{{- end }}
{{- if .Values.caBundles.existingConfigMap }}
- name: ca
  mountPath: /opt/capacity-room/ca
  readOnly: true
{{- end }}
{{- end -}}

{{/* The worker's memory limit in MiB, from a value the schema holds to Mi
or Gi. */}}
{{- define "capacity-room.workerMemoryMiB" -}}
{{- $memory := .Values.worker.resources.limits.memory -}}
{{- if hasSuffix "Gi" $memory -}}
{{- mul (trimSuffix "Gi" $memory | int) 1024 -}}
{{- else -}}
{{- trimSuffix "Mi" $memory | int -}}
{{- end -}}
{{- end -}}

{{/* profiles.toml: one [[profiles]] table per profile, each value written
as JSON, which TOML reads as the same string, number, boolean, or list. */}}
{{- define "capacity-room.profilesToml" -}}
{{- range .Values.profiles }}
[[profiles]]
{{- range $key, $value := . }}
{{- if ne $key "thresholds" }}
{{ $key }} = {{ toJson $value }}
{{- end }}
{{- end }}
{{- with .thresholds }}
[profiles.thresholds]
{{- range $key, $value := . }}
{{ $key }} = {{ toJson $value }}
{{- end }}
{{- end }}
{{- end }}
{{- end -}}

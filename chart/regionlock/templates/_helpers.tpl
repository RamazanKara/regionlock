{{/*
Common labels applied to every Regionlock ClusterPolicy.
*/}}
{{- define "regionlock.labels" -}}
app.kubernetes.io/name: regionlock
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
regionlock.io/ruleset: eu-data-residency-v1
{{- end -}}

{{/*
Kyverno JMESPath expressions. These are emitted VERBATIM into the rendered
manifest for Kyverno to evaluate at admission time. Backtick raw strings keep
Helm from trying to interpret Kyverno's own {{ }} braces.
*/}}
{{- define "regionlock.regionExpr" -}}
{{ `{{ request.object.spec.nodeSelector."topology.kubernetes.io/region" }}` }}
{{- end -}}

{{- define "regionlock.regionExprOrEmpty" -}}
{{ `{{ request.object.spec.nodeSelector."topology.kubernetes.io/region" || '' }}` }}
{{- end -}}

{{- define "regionlock.serviceTypeOrEmpty" -}}
{{ `{{ request.object.spec.type || '' }}` }}
{{- end -}}

{{- define "regionlock.egressCidrs" -}}
{{ `{{ request.object.spec.egress[].to[].ipBlock.cidr }}` }}
{{- end -}}

{{/*
Reusable exclude block (namespaces exempt from a policy). Include with the
excludeNamespaces list as the context: {{ include "regionlock.exclude" .Values.excludeNamespaces }}
*/}}
{{- define "regionlock.exclude" -}}
{{- if . }}
exclude:
  any:
    - resources:
        namespaces:
        {{- toYaml . | nindent 10 }}
{{- end }}
{{- end -}}

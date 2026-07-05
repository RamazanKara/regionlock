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

{{/* Region values pinned via required nodeAffinity In-terms (flattened list).
     The `[]` after the filter flattens the matchExpression projection so the
     result is a flat list of region strings, not a list-of-lists. */}}
{{- define "regionlock.regionAffinityValues" -}}
{{ `{{ request.object.spec.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[].matchExpressions[?key=='topology.kubernetes.io/region' && operator=='In'][].values[] }}` }}
{{- end -}}

{{/* Count of region values pinned via required nodeAffinity In-terms. */}}
{{- define "regionlock.regionAffinityCount" -}}
{{ `{{ length(request.object.spec.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[].matchExpressions[?key=='topology.kubernetes.io/region' && operator=='In'][].values[] || '') }}` }}
{{- end -}}

{{/* Count of Service externalIPs ('' default has length 0 for absent/empty). */}}
{{- define "regionlock.externalIPsCount" -}}
{{ `{{ length(request.object.spec.externalIPs || '') }}` }}
{{- end -}}

{{/* Count of egress CIDRs that are a default route (/0 suffix, any spelling).
     The pipe resets the projection so the filter's @ binds to each CIDR string. */}}
{{- define "regionlock.openEgressCount" -}}
{{ `{{ length(request.object.spec.egress[].to[].ipBlock.cidr | [?ends_with(@, '/0')] || '') }}` }}
{{- end -}}

{{/* Total egress rules, and egress rules that have a peer selector (`to`). */}}
{{- define "regionlock.egressRuleCount" -}}
{{ `{{ length(request.object.spec.egress[] || '') }}` }}
{{- end -}}
{{- define "regionlock.egressWithPeersCount" -}}
{{ `{{ length(request.object.spec.egress[?to] || '') }}` }}
{{- end -}}

{{/*
Gatekeeper enforcementAction derived from the shared Enforce/Audit value.
*/}}
{{- define "regionlock.gatekeeperAction" -}}
{{- if eq .Values.enforcementAction "Audit" -}}dryrun{{- else -}}deny{{- end -}}
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

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

{{/* Count of egress CIDRs that are a default route (/0) or default-route half
     (/1 — catches the 0.0.0.0/1 + 128.0.0.0/1 split that together cover the whole
     space). The pipe resets the projection so the filter's @ binds to each CIDR. */}}
{{- define "regionlock.openEgressCount" -}}
{{ `{{ length(request.object.spec.egress[].to[].ipBlock.cidr | [?ends_with(@, '/0') || ends_with(@, '/1')] || '') }}` }}
{{- end -}}

{{/* PVC's storageClassName (or '' when absent). */}}
{{- define "regionlock.storageClassName" -}}
{{ `{{ request.object.spec.storageClassName || '' }}` }}
{{- end -}}

{{/* The CMK annotation value (or '') — the annotation key is injected between two
     literal-brace fragments so the dynamic key lands inside the JMESPath. */}}
{{- define "regionlock.cmkAnnotationExpr" -}}
{{ `{{ request.object.metadata.annotations."` }}{{ .Values.cmkAnnotation }}{{ `" || '' }}` }}
{{- end -}}

{{/* The encryption label value (or ''). */}}
{{- define "regionlock.encryptionLabelExpr" -}}
{{ `{{ request.object.metadata.labels."` }}{{ .Values.encryptionLabel }}{{ `" || '' }}` }}
{{- end -}}

{{/* Total egress rules, and egress rules that have a peer selector (`to`). */}}
{{- define "regionlock.egressRuleCount" -}}
{{ `{{ length(request.object.spec.egress[] || '') }}` }}
{{- end -}}
{{- define "regionlock.egressWithPeersCount" -}}
{{ `{{ length(request.object.spec.egress[?to] || '') }}` }}
{{- end -}}

{{/*
NOTE ON GATEKEEPER INSTALL ORDERING: Gatekeeper reconciles each ConstraintTemplate
into its backing CRD asynchronously, so on a COLD install the Constraint CR can be
applied before its CRD is Established ("no matches for kind"). We deliberately keep
both the ConstraintTemplate and the Constraint as NORMAL release resources (no Helm
hooks) so that `helm upgrade` patches them in place — a hook with a delete policy
would tear the enforcing Constraint down and recreate it, opening a fail-open window
on every upgrade. For a cold install, apply the chart, wait for the CRDs to be
Established, then apply once more (see docs/installation.md); the e2e workflow does
exactly this.
*/}}

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

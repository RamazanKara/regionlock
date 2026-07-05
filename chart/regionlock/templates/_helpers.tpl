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
Constraint hook annotations. Gatekeeper reconciles a ConstraintTemplate into its
backing CRD asynchronously, so a Constraint CR applied in the same pass fails with
"no matches for kind". Applying Constraints as post-install/post-upgrade hooks
lets the ConstraintTemplate CRDs become Established first. On uninstall, deleting
the ConstraintTemplate (a normal release resource) cascade-removes the Constraint.
*/}}
{{- define "regionlock.constraintHookAnnotations" -}}
helm.sh/hook: post-install,post-upgrade
helm.sh/hook-weight: "5"
helm.sh/hook-delete-policy: before-hook-creation
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

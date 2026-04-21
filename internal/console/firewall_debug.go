package console

import (
	"fmt"
)

func firewallDebugf(enabled bool, format string, args ...interface{}) {
	if !enabled {
		return
	}
	fmt.Printf("🧪 FWDEBUG | "+format+"\n", args...)
}

func logFirewallSubmitDiagnostics(enabled bool, diag map[string]interface{}) {
	if !enabled {
		return
	}
	if len(diag) == 0 {
		firewallDebugf(true, "submit diag empty")
		return
	}

	if formFound, ok := diag["formFound"].(bool); ok {
		firewallDebugf(true, "formFound=%t", formFound)
	}
	if action, ok := diag["formAction"].(string); ok && action != "" {
		firewallDebugf(true, "formAction=%s", action)
	}
	if route, ok := diag["formRoute"].(string); ok && route != "" {
		firewallDebugf(true, "formRoute=%s", route)
	}
	if currentURL, ok := diag["currentURL"].(string); ok && currentURL != "" {
		firewallDebugf(true, "currentURL=%s", currentURL)
	}
	if count, ok := diag["existingUniqueCount"].(float64); ok {
		firewallDebugf(true, "existingUniqueCount=%.0f", count)
	}
	if addUnique, ok := diag["addUnique"].(string); ok && addUnique != "" {
		firewallDebugf(true, "addUnique=%s", addUnique)
	}
	if addUniqueMask, ok := diag["addUniqueMask"].(string); ok && addUniqueMask != "" {
		firewallDebugf(true, "addUniqueMask=%s", addUniqueMask)
	}
	if removeUnique, ok := diag["removeUnique"].(string); ok && removeUnique != "" {
		firewallDebugf(true, "removeUnique=%s", removeUnique)
	}
	if removeUniqueMask, ok := diag["removeUniqueMask"].(string); ok && removeUniqueMask != "" {
		firewallDebugf(true, "removeUniqueMask=%s", removeUniqueMask)
	}
	if removeIndexes, ok := diag["removeIndexes"].([]interface{}); ok && len(removeIndexes) > 0 {
		firewallDebugf(true, "removeIndexes=%v", removeIndexes)
	}
	if removeMatchMode, ok := diag["removeMatchMode"].(string); ok && removeMatchMode != "" {
		firewallDebugf(true, "removeMatchMode=%s", removeMatchMode)
	}
	if removeRowCandidateCount, ok := diag["removeRowCandidateCount"].(float64); ok {
		firewallDebugf(true, "removeRowCandidateCount=%.0f", removeRowCandidateCount)
	}
	if removeTargetNormalized, ok := diag["removeTargetNormalized"].(string); ok && removeTargetNormalized != "" {
		firewallDebugf(true, "removeTargetNormalized=%s", removeTargetNormalized)
	}
	if totalRuleRows, ok := diag["totalRuleRows"].(float64); ok {
		firewallDebugf(true, "totalRuleRows=%.0f", totalRuleRows)
	}
	if removedCount, ok := diag["removedCount"].(float64); ok {
		firewallDebugf(true, "removedCount=%.0f", removedCount)
	}
	if targets, ok := diag["targets"].([]interface{}); ok && len(targets) > 0 {
		firewallDebugf(true, "internationalTargets=%v", targets)
	}
	if targets, ok := diag["botTargets"].([]interface{}); ok && len(targets) > 0 {
		firewallDebugf(true, "botTargets=%v", targets)
	}
	if before, ok := diag["beforeInternational"].([]interface{}); ok {
		firewallDebugf(true, "beforeInternational=%v", before)
	}
	if original, ok := diag["originalInternational"].([]interface{}); ok && len(original) > 0 {
		firewallDebugf(true, "originalInternational=%v", original)
	}
	if status, ok := diag["beforeInternationalFetchStatus"].(float64); ok {
		firewallDebugf(true, "beforeInternationalFetchStatus=%.0f", status)
	}
	if detected, ok := diag["beforeInternationalDetected"].([]interface{}); ok {
		firewallDebugf(true, "beforeInternationalDetected=%v", detected)
	}
	if preview, ok := diag["beforeInternationalFetchBodyPreview"].(string); ok && preview != "" {
		firewallDebugf(true, "beforeInternationalFetchBodyPreview=%q", preview)
	}
	if err, ok := diag["beforeInternationalFetchError"].(string); ok && err != "" {
		firewallDebugf(true, "beforeInternationalFetchError=%s", err)
	}
	if after, ok := diag["afterInternational"].([]interface{}); ok {
		firewallDebugf(true, "afterInternational=%v", after)
	}
	if submitted, ok := diag["submittedInternational"].([]interface{}); ok {
		firewallDebugf(true, "submittedInternational=%v", submitted)
	}
	if before, ok := diag["beforeBot"].([]interface{}); ok {
		firewallDebugf(true, "beforeBot=%v", before)
	}
	if status, ok := diag["beforeBotFetchStatus"].(float64); ok {
		firewallDebugf(true, "beforeBotFetchStatus=%.0f", status)
	}
	if detected, ok := diag["beforeBotDetected"].([]interface{}); ok {
		firewallDebugf(true, "beforeBotDetected=%v", detected)
	}
	if preview, ok := diag["beforeBotFetchBodyPreview"].(string); ok && preview != "" {
		firewallDebugf(true, "beforeBotFetchBodyPreview=%q", preview)
	}
	if err, ok := diag["beforeBotFetchError"].(string); ok && err != "" {
		firewallDebugf(true, "beforeBotFetchError=%s", err)
	}
	if after, ok := diag["afterBot"].([]interface{}); ok {
		firewallDebugf(true, "afterBot=%v", after)
	}
	if submitted, ok := diag["submittedBot"].([]interface{}); ok {
		firewallDebugf(true, "submittedBot=%v", submitted)
	}
	if filled, ok := diag["inboundUniqueFilled"].(float64); ok {
		firewallDebugf(true, "inboundUniqueFilled=%.0f", filled)
	}
	if filled, ok := diag["outboundUniqueFilled"].(float64); ok {
		firewallDebugf(true, "outboundUniqueFilled=%.0f", filled)
	}
	if count, ok := diag["inboundSubmitRows"].(float64); ok {
		firewallDebugf(true, "inboundSubmitRows=%.0f", count)
	}
	if count, ok := diag["outboundSubmitRows"].(float64); ok {
		firewallDebugf(true, "outboundSubmitRows=%.0f", count)
	}
	if count, ok := diag["botSubmitRows"].(float64); ok {
		firewallDebugf(true, "botSubmitRows=%.0f", count)
	}
	if count, ok := diag["internationalSubmitRows"].(float64); ok {
		firewallDebugf(true, "internationalSubmitRows=%.0f", count)
	}
	if count, ok := diag["submitParamCount"].(float64); ok {
		firewallDebugf(true, "submitParamCount=%.0f", count)
	}
	if preview, ok := diag["submitParamPreview"].([]interface{}); ok && len(preview) > 0 {
		max := len(preview)
		if max > 20 {
			max = 20
		}
		firewallDebugf(true, "submitParamPreview=%v", preview[:max])
	}
	if keyCounts, ok := diag["submitKeyCounts"].(map[string]interface{}); ok && len(keyCounts) > 0 {
		firewallDebugf(true, "submitKeyCounts=%v", keyCounts)
	}
	if removedRows, ok := diag["removedRows"].([]interface{}); ok && len(removedRows) > 0 {
		firewallDebugf(true, "removedRows=%v", removedRows)
	}
	if sectionFieldCount, ok := diag["sectionFieldCount"].(float64); ok {
		firewallDebugf(true, "sectionFieldCount=%.0f", sectionFieldCount)
	}
	if sectionFieldNames, ok := diag["sectionFieldNames"].([]interface{}); ok && len(sectionFieldNames) > 0 {
		firewallDebugf(true, "sectionFieldNames=%v", sectionFieldNames)
	}
	if sectionCountsBefore, ok := diag["sectionCountsBefore"].(map[string]interface{}); ok && len(sectionCountsBefore) > 0 {
		firewallDebugf(true, "sectionCountsBefore=%v", sectionCountsBefore)
	}
	if sectionCountsAfter, ok := diag["sectionCountsAfter"].(map[string]interface{}); ok && len(sectionCountsAfter) > 0 {
		firewallDebugf(true, "sectionCountsAfter=%v", sectionCountsAfter)
	}
	for _, prefix := range []string{"inbound", "outbound", "bot"} {
		if status, ok := diag[prefix+"HydrateStatus"].(float64); ok {
			firewallDebugf(true, "%sHydrateStatus=%.0f", prefix, status)
		}
		if count, ok := diag[prefix+"HydrateCount"].(float64); ok {
			firewallDebugf(true, "%sHydrateCount=%.0f", prefix, count)
		}
		if count, ok := diag[prefix+"HydrateRuleCount"].(float64); ok && count > 0 {
			firewallDebugf(true, "%sHydrateRuleCount=%.0f", prefix, count)
		}
		if count, ok := diag[prefix+"HydrateMissingIdxCount"].(float64); ok && count > 0 {
			firewallDebugf(true, "%sHydrateMissingIdxCount=%.0f", prefix, count)
		}
		if rows, ok := diag[prefix+"HydrateDataRows"].(float64); ok {
			firewallDebugf(true, "%sHydrateDataRows=%.0f", prefix, rows)
		}
		if byPanel, ok := diag[prefix+"HydrateByPanel"].(bool); ok && byPanel {
			firewallDebugf(true, "%sHydrateByPanel=true", prefix)
		}
		if count, ok := diag[prefix+"HydratePanelCount"].(float64); ok && count > 0 {
			firewallDebugf(true, "%sHydratePanelCount=%.0f", prefix, count)
		}
		if count, ok := diag[prefix+"HydratePanelScripts"].(float64); ok && count > 0 {
			firewallDebugf(true, "%sHydratePanelScripts=%.0f", prefix, count)
		}
		if skipped, ok := diag[prefix+"HydrateSkippedNoRows"].(bool); ok && skipped {
			firewallDebugf(true, "%sHydrateSkippedNoRows=true", prefix)
		}
		if preview, ok := diag[prefix+"HydrateBodyPreview"].(string); ok && preview != "" {
			firewallDebugf(true, "%sHydrateBodyPreview=%q", prefix, preview)
		}
	}
	if submitURL, ok := diag["submitURL"].(string); ok && submitURL != "" {
		firewallDebugf(true, "submitURL=%s", submitURL)
	}
	if status, ok := diag["status"].(float64); ok {
		firewallDebugf(true, "submitStatus=%.0f", status)
	}
	if bodyPreview, ok := diag["bodyPreview"].(string); ok && bodyPreview != "" {
		firewallDebugf(true, "submitBodyPreview=%q", bodyPreview)
	}
	if looksLikeLogin, ok := diag["looksLikeLogin"].(bool); ok {
		firewallDebugf(true, "looksLikeLogin=%t", looksLikeLogin)
	}
	if hasErrorKeyword, ok := diag["hasErrorKeyword"].(bool); ok {
		firewallDebugf(true, "hasErrorKeyword=%t", hasErrorKeyword)
	}
	if hasSuccessKeyword, ok := diag["hasSuccessKeyword"].(bool); ok {
		firewallDebugf(true, "hasSuccessKeyword=%t", hasSuccessKeyword)
	}
	if tail, ok := diag["existingUniqueTail"].([]interface{}); ok && len(tail) > 0 {
		firewallDebugf(true, "existingUniqueTail=%v", tail)
	}
	if required, ok := diag["requiredSnapshot"].(map[string]interface{}); ok && len(required) > 0 {
		firewallDebugf(true, "requiredSnapshot=%v", required)
	}
}

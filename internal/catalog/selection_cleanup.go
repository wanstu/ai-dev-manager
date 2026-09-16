package catalog

func removeCatalogSelections(values []string, removed map[string]struct{}) []string {
	if len(values) == 0 || len(removed) == 0 {
		return values
	}
	kept := values[:0]
	for _, value := range values {
		if _, drop := removed[value]; drop {
			continue
		}
		kept = append(kept, value)
	}
	return kept
}

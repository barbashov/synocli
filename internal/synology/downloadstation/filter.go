package downloadstation

// FilterTasks returns tasks matching the given ID and status sets.
// An empty set is treated as "match all".
func FilterTasks(tasks []Task, idSet, statusSet map[string]struct{}) []Task {
	out := make([]Task, 0, len(tasks))
	for _, t := range tasks {
		if len(idSet) > 0 {
			if _, ok := idSet[t.ID]; !ok {
				continue
			}
		}
		if len(statusSet) > 0 {
			if _, ok := statusSet[NormalizeStatus(t.Status)]; !ok {
				continue
			}
		}
		out = append(out, t)
	}
	return out
}

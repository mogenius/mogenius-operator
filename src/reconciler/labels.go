package reconciler

func getDefaultLabels() map[string]string {
	return map[string]string{
		"app.kubernetes.io/managed-by": "mogenius-operator",
	}
}

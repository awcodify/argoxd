package argocd

import (
	"errors"
	"net/http"
	"strings"

	"github.com/awcodify/argoxd/internal/explorer"
)

// generatorKinds are the generators an ApplicationSet can list.
var generatorKinds = []string{"list", "clusters", "git", "scmProvider", "clusterDecisionResource", "pullRequest", "matrix", "merge", "plugin"}

// generatorSummary names each generator of an ApplicationSet, with the
// generators nested in a matrix or merge, e.g. "matrix(git, clusters)".
func generatorSummary(generators []any) []string {
	names := make([]string, 0, len(generators))
	for _, generator := range generators {
		fields, _ := generator.(map[string]any)
		names = append(names, generatorName(fields))
	}
	return names
}

func generatorName(fields map[string]any) string {
	for _, kind := range generatorKinds {
		value, found := fields[kind]
		if !found {
			continue
		}
		if kind != "matrix" && kind != "merge" {
			return kind
		}
		inner, _ := value.(map[string]any)
		nested, _ := inner["generators"].([]any)
		return kind + "(" + strings.Join(generatorSummary(nested), ", ") + ")"
	}
	return "unknown"
}

// appSetCondition is a condition in an ApplicationSet's status.
type appSetCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// applicationSetProblems keeps the conditions that say something is wrong:
// ErrorOccurred when it is true, and the others when they are false.
func applicationSetProblems(conditions []appSetCondition) []explorer.Condition {
	var problems []explorer.Condition
	for _, condition := range conditions {
		failing := condition.Status == "False"
		if condition.Type == "ErrorOccurred" {
			failing = condition.Status == "True"
		}
		if failing {
			problems = append(problems, explorer.Condition{Type: condition.Type, Message: condition.Message})
		}
	}
	return problems
}

// httpStatusError is a response from Argo CD that is not a success.
type httpStatusError struct {
	message string
	code    int
}

func (e *httpStatusError) Error() string { return e.message }

// unavailable reports whether Argo CD refused a request or does not have what
// it asks for, as when it has no ApplicationSets or the token may not list them.
func unavailable(err error) bool {
	var status *httpStatusError
	return errors.As(err, &status) && (status.code == http.StatusForbidden || status.code == http.StatusNotFound)
}

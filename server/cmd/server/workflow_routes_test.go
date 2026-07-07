package main

import (
	"net/http"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/multica-ai/multica/server/internal/analytics"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/realtime"
)

func TestWorkflowIssueFirstControlRoutesAreDebugOnly(t *testing.T) {
	router := NewRouter(nil, realtime.NewHub(), events.New(), analytics.NoopClient{}, nil)

	routes := map[string]bool{}
	if err := chi.Walk(router, func(method string, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		routes[method+" "+route] = true
		return nil
	}); err != nil {
		t.Fatalf("walk routes: %v", err)
	}

	for _, route := range []string{
		"GET /api/issues/{id}/workflow-context",
		"POST /api/workflow-cases/{caseId}/runs",
		"GET /api/workflow-cases/{caseId}/current-run",
		"POST /api/workflow-cases/{caseId}/runs/{runId}/cancel",
		"PATCH /api/workflow-runs/{runId}",
		"POST /api/workflow-runs/{runId}/main-node-task",
		"POST /api/workflow-runs/{runId}/nodes/{nodeId}/events",
	} {
		if !routes[route] {
			t.Fatalf("expected route %q to be registered", route)
		}
	}

	for _, route := range []string{
		"GET /api/issues/{id}/workflow-run",
		"POST /api/issues/{id}/workflow-run/start",
		"POST /api/issues/{id}/runtime-workflows",
		"POST /api/workflow-runs/",
		"POST /api/workflow-runs/{runId}/cancel",
		"POST /api/workflow-runs/{runId}/continue",
	} {
		if routes[route] {
			t.Fatalf("workflow control route %q must not be registered as a normal product route", route)
		}
	}

	for _, route := range []string{
		"GET /api/internal/debug/workflow/issues/{id}/workflow-run",
		"POST /api/internal/debug/workflow/issues/{id}/workflow-run/start",
		"POST /api/internal/debug/workflow/issues/{id}/runtime-workflows",
		"POST /api/internal/debug/workflow/runs/",
		"POST /api/internal/debug/workflow/runs/{runId}/cancel",
		"POST /api/internal/debug/workflow/runs/{runId}/continue",
	} {
		if !routes[route] {
			t.Fatalf("expected deprecated debug compatibility route %q to be registered", route)
		}
		if !strings.Contains(route, "/internal/debug/") {
			t.Fatalf("debug compatibility route %q is not clearly internal/debug", route)
		}
	}
}

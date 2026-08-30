package main

import (
	"os"
	"strings"
	"testing"
)

func TestProductionDockerfileUsesPinnedCachedBuildInputs(t *testing.T) {
	contents, err := os.ReadFile("../../Dockerfile")
	if err != nil {
		t.Fatal(err)
	}
	text := string(contents)
	if !strings.Contains(text, "ARG GO_VERSION=1.26.6") {
		t.Fatal("production Go patch release must remain explicit")
	}
	for _, forbidden := range []string{"@latest", "swag init", "go install github.com/swaggo/swag"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("production Dockerfile contains unpinned build-time generator %q", forbidden)
		}
	}
}

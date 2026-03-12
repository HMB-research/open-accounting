package docs

import (
	"testing"

	"github.com/swaggo/swag"
)

func TestSwaggerRegistration(t *testing.T) {
	if SwaggerInfo.InstanceName() != "swagger" {
		t.Fatalf("unexpected instance name: %s", SwaggerInfo.InstanceName())
	}

	if SwaggerInfo.ReadDoc() == "" {
		t.Fatal("expected generated swagger document to be available")
	}

	if spec := swag.GetSwagger(SwaggerInfo.InstanceName()); spec == nil {
		t.Fatal("expected swagger spec to be registered")
	}
}

package docs

import (
	"encoding/json"
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

func TestSwaggerDocumentsInventoryOperations(t *testing.T) {
	var spec struct {
		Paths map[string]map[string]any `json:"paths"`
	}
	if err := json.Unmarshal([]byte(SwaggerInfo.ReadDoc()), &spec); err != nil {
		t.Fatalf("decode swagger doc: %v", err)
	}

	expected := map[string]string{
		"/tenants/{tenantID}/products/{productID}/stock-levels": "get",
		"/tenants/{tenantID}/products/{productID}/movements":    "get",
		"/tenants/{tenantID}/inventory/adjust":                  "post",
		"/tenants/{tenantID}/inventory/stock-import":            "post",
		"/tenants/{tenantID}/inventory/transfer":                "post",
		"/tenants/{tenantID}/inventory/reserve":                 "post",
		"/tenants/{tenantID}/inventory/release":                 "post",
		"/tenants/{tenantID}/inventory/valuation":               "get",
	}
	for path, method := range expected {
		methods, ok := spec.Paths[path]
		if !ok {
			t.Fatalf("expected swagger path %s", path)
		}
		if _, ok := methods[method]; !ok {
			t.Fatalf("expected swagger method %s %s", method, path)
		}
	}
}

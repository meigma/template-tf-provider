package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/provider"
)

func TestMetadataReportsTypeNameAndVersion(t *testing.T) {
	t.Parallel()

	p := New("0.1.0")()

	var resp provider.MetadataResponse
	p.Metadata(context.Background(), provider.MetadataRequest{}, &resp)

	if got, want := resp.TypeName, "example"; got != want {
		t.Fatalf("TypeName = %q, want %q", got, want)
	}
	if got, want := resp.Version, "0.1.0"; got != want {
		t.Fatalf("Version = %q, want %q", got, want)
	}
}

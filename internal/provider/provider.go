// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// Ensure FhirRestProvider satisfies various provider interfaces.
var _ provider.Provider = &FhirRestProvider{}

// FhirRestProvider defines the provider implementation.
type FhirRestProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// FhirRestProviderModel describes the provider data model.
type FhirRestProviderModel struct {
	FhirBaseUrl    types.String `tfsdk:"fhir_base_url"`
	DefaultHeaders types.Map    `tfsdk:"default_headers"`
}

type ProviderSettings struct {
	FhirBaseUrl    string
	DefaultHeaders map[string]string
	Client         *http.Client
}

func (p *FhirRestProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "fhirrest"
	resp.Version = p.version
}

func (p *FhirRestProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"fhir_base_url": schema.StringAttribute{
				MarkdownDescription: "The Base URL of the fhir server",
				Required:            true,
			},
			"default_headers": schema.MapAttribute{
				ElementType:         basetypes.StringType{},
				MarkdownDescription: "The headers of the http requests",
				Optional:            true,
			},
		},
	}
}

func (p *FhirRestProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data FhirRestProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	headers := make(map[string]string)
	data.DefaultHeaders.ElementsAs(ctx, &headers, true)
	settings := &ProviderSettings{
		FhirBaseUrl:    data.FhirBaseUrl.ValueString(),
		DefaultHeaders: headers,
		Client:         http.DefaultClient,
	}

	// Example client configuration for data sources and resources
	resp.DataSourceData = settings
	resp.ResourceData = settings
}

func (p *FhirRestProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewFhirResource,
	}
}

func (p *FhirRestProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewFhirResourceDataSource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &FhirRestProvider{
			version: version,
		}
	}
}

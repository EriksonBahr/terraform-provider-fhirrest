// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &FhirResourceDataSource{}

func NewFhirResourceDataSource() datasource.DataSource {
	return &FhirResourceDataSource{}
}

// FhirResourceDataSource defines the data source implementation.
type FhirResourceDataSource struct {
	providerSettings *ProviderSettings
}

// FhirResourceDataSourceModel describes the data source data model.
type FhirResourceDataSourceModel struct {
	ResourceId types.String `tfsdk:"resource_id"`
	Resource   types.String `tfsdk:"resource"`
}

func (d *FhirResourceDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_fhir_resource"
}

func (d *FhirResourceDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "This data source is able to read a fhir resource and return it as a json",

		Attributes: map[string]schema.Attribute{
			"resource_id": schema.StringAttribute{
				MarkdownDescription: "The id of the fhir resource, example Medication/08146022-932a-4001-9fe4-928382855ddf",
				Required:            true,
			},
			"resource": schema.StringAttribute{
				MarkdownDescription: "The fhir json as string",
				Computed:            true,
			},
		},
	}
}

func (d *FhirResourceDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	ok := true
	d.providerSettings, ok = req.ProviderData.(*ProviderSettings)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *ProviderSettings, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}
}

func (d *FhirResourceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data FhirResourceDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	body, shouldReturn := ReadFhirResource(d.providerSettings, data.ResourceId.ValueString(), &resp.Diagnostics)
	if shouldReturn {
		return
	}

	data.Resource = types.StringValue(string(body))

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

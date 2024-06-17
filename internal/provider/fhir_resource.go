// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &FhirResource{}
var _ resource.ResourceWithImportState = &FhirResource{}

func NewFhirResource() resource.Resource {
	return &FhirResource{}
}

// FhirResource defines the resource implementation.
type FhirResource struct {
	providerSettings     *ProviderSettings
	fhirResourceSettings FhirResourceSettings
}

type FhirResourceSettings struct {
	FhirResourceFilePath string
	FhirBaseUrl          *string
}

type FhirResourceModel struct {
	// from model
	FilePath    types.String `tfsdk:"file_path"`
	FileSha256  types.String `tfsdk:"file_sha256"`
	FhirBaseUrl types.String `tfsdk:"fhir_base_url"`

	//actual state
	ResourceId     types.String `tfsdk:"resource_id"`
	ResponseSha256 types.String `tfsdk:"response_sha256"`
}

func (r *FhirResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_fhir_resource"
}

func (r *FhirResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "This represents a fhir resource in the FHIR server",

		Attributes: map[string]schema.Attribute{
			"file_path": schema.StringAttribute{
				MarkdownDescription: "The path of the file containing a fhir resource",
				Required:            true,
			},
			"file_sha256": schema.StringAttribute{
				MarkdownDescription: "The sha256 of the file. Not internally used, but useful to trigger updates when the file is updated",
				Optional:            true,
			},
			"fhir_base_url": schema.StringAttribute{
				MarkdownDescription: "The Base URL of the fhir server. Overrides the value set in the provider (if any set)",
				Optional:            true,
			},
			"resource_id": schema.StringAttribute{
				MarkdownDescription: "The id of the resource that was saved in the fhir server",
				Computed:            true,
			},
			"response_sha256": schema.StringAttribute{
				MarkdownDescription: "The sha256 of the response of the fhir server.",
				Computed:            true,
			},
		},
	}
}

func (r *FhirResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	ok := true
	r.providerSettings, ok = req.ProviderData.(*ProviderSettings)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *ProviderSettings, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}
}

func (r *FhirResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data FhirResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	r.fhirResourceSettings = FhirResourceSettings{FhirResourceFilePath: data.FilePath.ValueString(), FhirBaseUrl: data.FhirBaseUrl.ValueStringPointer()}

	if resp.Diagnostics.HasError() {
		return
	}

	body, responseJson, resourceType := persistFhirResource(ctx, r, nil, &resp.Diagnostics)
	if responseJson == nil {
		return
	}

	hash := sha256.Sum256(body)
	hashString := hex.EncodeToString(hash[:])

	id := responseJson["id"].(string)
	data.ResourceId = types.StringValue(fmt.Sprintf("%s/%s", *resourceType, id))
	data.ResponseSha256 = types.StringValue(hashString)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func persistFhirResource(ctx context.Context, fhirResource *FhirResource, resourceId *string, diag *diag.Diagnostics) ([]byte, map[string]interface{}, *string) {
	fileContent := readFileContent(fhirResource.fhirResourceSettings.FhirResourceFilePath, diag)
	if fileContent == nil {
		return nil, nil, nil
	}

	var fileContentJson map[string]interface{}
	if err := json.Unmarshal(fileContent, &fileContentJson); err != nil {
		diag.AddError(fmt.Sprintf("failed to unmarshal JSON file %s", fhirResource.fhirResourceSettings.FhirResourceFilePath), err.Error())
		return nil, nil, nil
	}
	resourceType, ok := fileContentJson["resourceType"]
	resourceTypeStr := fmt.Sprintf("%s", resourceType)
	if !ok {
		diag.AddError(fmt.Sprintf("property resourceType not found in json file %s", fhirResource.fhirResourceSettings.FhirResourceFilePath), "")
		return nil, nil, nil
	}

	baseUrl := fhirResource.providerSettings.FhirBaseUrl
	if fhirResource.fhirResourceSettings.FhirBaseUrl != nil {
		baseUrl = *fhirResource.fhirResourceSettings.FhirBaseUrl
	}
	url := fmt.Sprintf("%s/%s", baseUrl, resourceTypeStr)
	requestBody := fileContent
	requestMethod := "POST"
	if resourceId != nil {
		url = fmt.Sprintf("%s/%s", baseUrl, *resourceId)
		requestMethod = "PUT"
		parts := strings.Split(*resourceId, "/")
		fileContentJson["id"] = parts[len(parts)-1]
		requestBody, _ = json.Marshal(fileContentJson)
	}
	postRequest, err := http.NewRequest(requestMethod, url, bytes.NewBuffer(requestBody))
	if err != nil {
		diag.AddError("failed to create new request", err.Error())
		return nil, nil, nil
	}
	for key, value := range fhirResource.providerSettings.DefaultHeaders {
		postRequest.Header.Set(key, value)
	}
	postRequest.Header.Set("Content-Type", "application/json")

	postResponse, err := fhirResource.providerSettings.Client.Do(postRequest)
	if err != nil {
		diag.AddError(fmt.Sprintf("could not post the %s on the url %s", resourceType, url), err.Error())
		return nil, nil, nil
	}
	defer postResponse.Body.Close()

	body, _ := io.ReadAll(postResponse.Body)
	if postResponse.Status[0] != '2' {
		diag.AddError(fmt.Sprintf("the server returned an invalid status for the %s on the url %s: %s", resourceType, url, postResponse.Status), string(body))
		return nil, nil, nil
	}

	var responseJson map[string]interface{}
	if err := json.Unmarshal(body, &responseJson); err != nil {
		diag.AddError(fmt.Sprintf("failed to unmarshal response JSON of the resource %s", resourceType), err.Error())
		return nil, nil, nil
	}
	tflog.Debug(ctx, fmt.Sprintf("persisted the resource %s. Response: %s", resourceType, string(body)))
	return body, responseJson, &resourceTypeStr
}

func readFileContent(filePath string, diag *diag.Diagnostics) []byte {
	jsonFile, err := os.Open(filePath)
	if err != nil {
		diag.AddError(fmt.Sprintf("failed to read file %s", filePath), err.Error())
		return nil
	}
	defer jsonFile.Close()

	byteValue, err := io.ReadAll(jsonFile)
	if err != nil {
		diag.AddError(fmt.Sprintf("failed to open file %s", filePath), err.Error())
		return nil
	}
	return byteValue
}

func (r *FhirResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data FhirResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	r.fhirResourceSettings = FhirResourceSettings{FhirResourceFilePath: data.FilePath.ValueString(), FhirBaseUrl: data.FhirBaseUrl.ValueStringPointer()}

	body, shouldReturn := ReadFhirResource(r.providerSettings, r.fhirResourceSettings.FhirBaseUrl, data.ResourceId.ValueString(), &resp.Diagnostics)
	if shouldReturn {
		return
	}

	var responseJson map[string]interface{}
	if err := json.Unmarshal(body, &responseJson); err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("failed to unmarshal response JSON of the resource %s", data.ResourceId.ValueString()), err.Error())
		return
	}

	hash := sha256.Sum256(body)
	hashString := hex.EncodeToString(hash[:])

	id := responseJson["id"].(string)
	resourceType := responseJson["resourceType"].(string)
	data.ResourceId = types.StringValue(fmt.Sprintf("%s/%s", resourceType, id))
	data.ResponseSha256 = types.StringValue(hashString)

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *FhirResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state FhirResourceModel

	// Read Terraform state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	var data FhirResourceModel
	// Read Terraform state data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	r.fhirResourceSettings = FhirResourceSettings{FhirResourceFilePath: data.FilePath.ValueString(), FhirBaseUrl: data.FhirBaseUrl.ValueStringPointer()}

	body, responseJson, resourceType := persistFhirResource(ctx, r, state.ResourceId.ValueStringPointer(), &resp.Diagnostics)
	if responseJson == nil {
		return
	}

	hash := sha256.Sum256(body)
	hashString := hex.EncodeToString(hash[:])

	id := responseJson["id"].(string)
	state.ResourceId = types.StringValue(fmt.Sprintf("%s/%s", *resourceType, id))
	state.ResponseSha256 = types.StringValue(hashString)
	state.FilePath = data.FilePath
	state.FileSha256 = data.FileSha256

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *FhirResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data FhirResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	r.fhirResourceSettings = FhirResourceSettings{FhirResourceFilePath: data.FilePath.ValueString(), FhirBaseUrl: data.FhirBaseUrl.ValueStringPointer()}

	baseUrl := r.providerSettings.FhirBaseUrl
	if r.fhirResourceSettings.FhirBaseUrl != nil {
		baseUrl = *r.fhirResourceSettings.FhirBaseUrl
	}
	url := fmt.Sprintf("%s/%s", baseUrl, data.ResourceId.ValueString())
	deleteRequest, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("could not create the delete request using the URL %s", url), err.Error())
		return
	}
	for key, value := range r.providerSettings.DefaultHeaders {
		deleteRequest.Header.Set(key, value)
	}
	deleteRequest.Header.Set("Content-Type", "application/json")
	deleteResponse, err := r.providerSettings.Client.Do(deleteRequest)
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("could not delete the resource using the URL %s", url), err.Error())
		return
	}

	defer deleteResponse.Body.Close()

	body, _ := io.ReadAll(deleteResponse.Body)
	if deleteResponse.Status[0] != '2' {
		resp.Diagnostics.AddError(fmt.Sprintf("could not delete the resource using the URL %s.", url), fmt.Sprintf("Error code %s. Response: %s", deleteResponse.Status, string(body)))
		return
	}
}

func (r *FhirResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("resource_id"), req, resp)
}

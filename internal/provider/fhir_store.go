package provider

import (
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/diag"
)

func ReadFhirResource(providerSettings *ProviderSettings, resourceId string, diag *diag.Diagnostics) ([]byte, bool) {
	url := fmt.Sprintf("%s/%s", providerSettings.FhirBaseUrl, resourceId)
	getRequest, err := http.NewRequest("GET", url, nil)
	if err != nil {
		diag.AddError(fmt.Sprintf("could get the resource request using the URL %s", url), err.Error())
		return nil, true
	}
	for key, value := range providerSettings.DefaultHeaders {
		getRequest.Header.Set(key, value)
	}
	getRequest.Header.Set("Content-Type", "application/json")
	getResponse, err := providerSettings.Client.Do(getRequest)
	if err != nil {
		diag.AddError(fmt.Sprintf("could not delete the resource using the URL %s", url), err.Error())
		return nil, true
	}

	defer getResponse.Body.Close()

	body, _ := io.ReadAll(getResponse.Body)
	if getResponse.Status[0] != '2' {
		diag.AddError(fmt.Sprintf("could not get the resource using the URL %s.", url), fmt.Sprintf("Error code %s. Response: %s", getResponse.Status, string(body)))
		return nil, true
	}
	return body, false
}

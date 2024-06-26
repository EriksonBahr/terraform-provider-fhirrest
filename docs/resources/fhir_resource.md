---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "fhirrest_fhir_resource Resource - fhirrest"
subcategory: ""
description: |-
  This represents a fhir resource in the FHIR server
---

# fhirrest_fhir_resource (Resource)

This represents a fhir resource in the FHIR server



<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `file_path` (String) The path of the file containing a fhir resource

### Optional

- `fhir_base_url` (String) The Base URL of the fhir server. Overrides the value set in the provider (if any set)
- `file_sha256` (String) The sha256 of the file. Not internally used, but useful to trigger updates when the file is updated

### Read-Only

- `resource_id` (String) The id of the resource that was saved in the fhir server
- `response_sha256` (String) The sha256 of the response of the fhir server.

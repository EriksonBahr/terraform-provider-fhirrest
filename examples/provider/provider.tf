provider "fhirrest" {
  fhir_base_url = "http://hapi.fhir.org/baseR4"
  default_headers = {
    Content-Type = "application/json"
    Accept       = "application/fhir+json;fhirVersion=4.0"
  }
}
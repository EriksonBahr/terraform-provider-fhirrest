resource "fhirrest_fhir_resource" "patient" {
  file_path   = "${path.cwd}/patient.json"
  file_sha256 = sha256(file("${path.cwd}/patient.json"))
}

locals {
  project_name = "ballast-terraform-sample"
}

output "project_name" {
  description = "Example output to keep the sample root realistic."
  value       = local.project_name
}

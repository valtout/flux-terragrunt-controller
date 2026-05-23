# Test fixture - Terragrunt configuration
# This simulates a real workload that the controller would manage

terraform {
  source = "../modules/example"
}

include {
  path = find_in_parent_folders()
}

inputs = {
  environment = "prod"
  region      = "somewhere"
}

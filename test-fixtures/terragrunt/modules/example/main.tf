terraform {
  required_version = ">= 1.0"

  required_providers {
    random = {
      source  = "hashicorp/random"
      version = "~> 3.0"
    }
  }
}

variable "environment" {
  description = "Environment name"
  type        = string
}

variable "region" {
  description = "Some region"
  type        = string
}

resource "random_string" "example" {
  length  = 8
  upper   = false
  special = false
}

output "environment" {
  value = var.environment
}

output "region" {
  value = var.region
}

output "random_suffix" {
  value = random_string.example.result
}

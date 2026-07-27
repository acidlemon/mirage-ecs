// A long-lived state that manages only the Route53 hosted zone.
// Apply this once, delegate the domain to the name servers in the output,
// and keep it while you create/destroy the main state repeatedly.

variable "domain" {
  type = string
}

variable "region" {
  type    = string
  default = "ap-northeast-1"
}

provider "aws" {
  region = var.region
}

terraform {
  required_version = ">= 1.8.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
  }
}

resource "aws_route53_zone" "main" {
  name = var.domain
}

output "zone_id" {
  value = aws_route53_zone.main.zone_id
}

output "name_servers" {
  value = aws_route53_zone.main.name_servers
}

variable "project" {
  type    = string
  default = "mirage-ecs"
}

variable "region" {
  type    = string
  default = "ap-northeast-1"
}

variable "domain" {
  type = string
}

provider "aws" {
  region = var.region
  default_tags {
    tags = {
      "env" = var.project
    }
  }
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

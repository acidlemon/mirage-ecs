provider "aws" {
  region = "ap-northeast-1"
  default_tags {
    tags = {
      "env" = "mirage-ecs"
    }
  }
}

terraform {
  required_version = "= 1.4.6"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "= 4.65.0"
    }
  }
}

data "aws_caller_identity" "current" {
}

variable "domain" {
  type = string
}

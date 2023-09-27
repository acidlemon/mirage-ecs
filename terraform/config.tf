variable "project" {
  type    = string
  default = "mirage-ecs"
}

provider "aws" {
  region = "ap-northeast-1"
  default_tags {
    tags = {
      "env" = "${var.project}"
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

variable "oauth_client_id" {
  type    = string
  default = ""
}

variable "oauth_client_secret" {
  type    = string
  default = ""
}

provider "http" {}

data "http" "oidc_configuration" {
  url = "https://accounts.google.com/.well-known/openid-configuration"
}

terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
  }
  required_version = ">= 1.12"
}

provider "aws" {
  region = var.region
  default_tags {
    tags = {
      Environment = "Test"
    }
  }
}

provider "aws" {
  alias  = "us_east_1"
  region = "us-east-1"
  default_tags {
    tags = {
      Purpose = "ACM"
    }
  }
}

provider "local" {}


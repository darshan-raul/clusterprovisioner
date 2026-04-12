terraform {
  required_version = ">= 1.6.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }

  # Infra uses LOCAL state — it is always-on and not ephemerally destroyed
  # If you prefer remote state, add an S3 backend here with a different key
}

provider "aws" {
  region = var.region
}

variable "region" {
  default = "ap-south-1"
}

variable "cluster_name" {
  default = "eks-on-demand"
}

variable "tf_state_bucket" {
  description = "S3 bucket for EKS Terraform state"
  default     = "eks-control-tf-state"
}

variable "tf_lock_table" {
  description = "DynamoDB table for EKS Terraform lock"
  default     = "eks-control-tf-lock"
}

# ---------------------------------------------------------------------------
# Variables
# ---------------------------------------------------------------------------

variable "aws_region" {
  type        = string
  default     = "ap-south-1"
  description = "Primary AWS region for the ACCIO platform"
}

variable "environment" {
  type        = string
  default     = "prod"
  description = "Deployment environment tag"
}

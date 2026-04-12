variable "cluster_name" {
  description = "EKS cluster name"
  type        = string
  default     = "eks-on-demand"
}

variable "region" {
  description = "AWS region"
  type        = string
  default     = "ap-south-1"
}

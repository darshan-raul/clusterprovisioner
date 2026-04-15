# ---------------------------------------------------------------------------
# KMS key — encrypts S3 buckets, Secrets Manager secrets, DynamoDB (optional)
# ---------------------------------------------------------------------------

resource "aws_kms_key" "accio" {
  description             = "ACCIO platform master encryption key"
  deletion_window_in_days = 30
  enable_key_rotation     = true

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      # Key owner — full access for root account
      {
        Sid    = "RootAccess"
        Effect = "Allow"
        Principal = { AWS = "arn:aws:iam::${local.account_id}:root" }
        Action   = "kms:*"
        Resource = "*"
      },
      # Lambda + CodeBuild roles can use the key for encrypt/decrypt
      {
        Sid    = "ServiceRolesEncryptDecrypt"
        Effect = "Allow"
        Principal = {
          AWS = [
            aws_iam_role.accio_lambda.arn,
            aws_iam_role.accio_codebuild.arn,
          ]
        }
        Action = [
          "kms:Decrypt",
          "kms:GenerateDataKey",
          "kms:DescribeKey",
        ]
        Resource = "*"
      },
    ]
  })

  tags = { Name = "accio-master-key" }
}

resource "aws_kms_alias" "accio" {
  name          = "alias/accio-master"
  target_key_id = aws_kms_key.accio.key_id
}
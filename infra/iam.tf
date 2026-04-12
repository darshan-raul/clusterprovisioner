# ─── IAM Role: cluster_control + auto_teardown ──────────────────────────────

resource "aws_iam_role" "lambda_control" {
  name = "eks-control-lambda-control-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "lambda.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })
}

resource "aws_iam_role_policy" "lambda_control_policy" {
  name = "eks-control-lambda-control-policy"
  role = aws_iam_role.lambda_control.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      { Effect = "Allow", Action = ["eks:*"], Resource = "*" },
      { Effect = "Allow", Action = ["ec2:*"], Resource = "*" },
      { Effect = "Allow", Action = ["iam:*"], Resource = "*" },
      {
        Effect   = "Allow"
        Action   = ["s3:*"]
        Resource = "arn:aws:s3:::${var.tf_state_bucket}/*"
      },
      {
        Effect   = "Allow"
        Action   = ["s3:ListBucket"]
        Resource = "arn:aws:s3:::${var.tf_state_bucket}"
      },
      {
        Effect   = "Allow"
        Action   = ["dynamodb:*"]
        Resource = "arn:aws:dynamodb:${var.region}:*:table/${var.tf_lock_table}"
      },
      {
        Effect = "Allow"
        Action = [
          "ssm:PutParameter",
          "ssm:GetParameter",
          "ssm:DeleteParameter"
        ]
        Resource = "arn:aws:ssm:${var.region}:*:parameter/eks-control/*"
      },
      { Effect = "Allow", Action = ["logs:*"], Resource = "*" }
    ]
  })
}

# ─── IAM Role: cluster_status ────────────────────────────────────────────────

resource "aws_iam_role" "lambda_status" {
  name = "eks-control-lambda-status-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "lambda.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })
}

resource "aws_iam_role_policy" "lambda_status_policy" {
  name = "eks-control-lambda-status-policy"
  role = aws_iam_role.lambda_status.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = ["eks:DescribeCluster"]
        Resource = "*"
      },
      {
        Effect   = "Allow"
        Action   = ["cloudwatch:GetMetricStatistics"]
        Resource = "*"
      },
      {
        Effect = "Allow"
        Action = [
          "ssm:GetParameter"
        ]
        Resource = "arn:aws:ssm:${var.region}:*:parameter/eks-control/*"
      },
      { Effect = "Allow", Action = ["logs:*"], Resource = "*" }
    ]
  })
}

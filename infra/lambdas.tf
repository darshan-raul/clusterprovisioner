data "archive_file" "cluster_control_zip" {
  type        = "zip"
  source_dir  = "${path.root}/../lambdas/cluster_control"
  output_path = "${path.root}/build/cluster_control.zip"
}

data "archive_file" "cluster_status_zip" {
  type        = "zip"
  source_dir  = "${path.root}/../lambdas/cluster_status"
  output_path = "${path.root}/build/cluster_status.zip"
}

data "archive_file" "auto_teardown_zip" {
  type        = "zip"
  source_dir  = "${path.root}/../lambdas/auto_teardown"
  output_path = "${path.root}/build/auto_teardown.zip"
}

# ─── Lambda: cluster_control ────────────────────────────────────────────────

resource "aws_lambda_function" "cluster_control" {
  function_name    = "eks-cluster-control"
  role             = aws_iam_role.lambda_control.arn
  handler          = "handler.lambda_handler"
  runtime          = "python3.12"
  timeout          = 900 # 15 minutes (EKS apply can take 8-10 min)
  memory_size      = 512
  filename         = data.archive_file.cluster_control_zip.output_path
  source_code_hash = data.archive_file.cluster_control_zip.output_base64sha256

  environment {
    variables = {
      TF_STATE_BUCKET = var.tf_state_bucket
      TF_LOCK_TABLE   = var.tf_lock_table
      AWS_REGION      = var.region
    }
  }

  # NOTE: The Terraform binary must be provided as a Lambda Layer.
  # See README.md for instructions on building the layer.
  # layers = [aws_lambda_layer_version.terraform_binary.arn]

  tags = { Project = "eks-control" }
}

# ─── Lambda: cluster_status ─────────────────────────────────────────────────

resource "aws_lambda_function" "cluster_status" {
  function_name    = "eks-cluster-status"
  role             = aws_iam_role.lambda_status.arn
  handler          = "handler.lambda_handler"
  runtime          = "python3.12"
  timeout          = 30
  memory_size      = 256
  filename         = data.archive_file.cluster_status_zip.output_path
  source_code_hash = data.archive_file.cluster_status_zip.output_base64sha256

  environment {
    variables = {
      CLUSTER_NAME = var.cluster_name
    }
  }

  tags = { Project = "eks-control" }
}

# ─── Lambda: auto_teardown ──────────────────────────────────────────────────

resource "aws_lambda_function" "auto_teardown" {
  function_name    = "eks-auto-teardown"
  role             = aws_iam_role.lambda_control.arn
  handler          = "handler.lambda_handler"
  runtime          = "python3.12"
  timeout          = 900
  memory_size      = 512
  filename         = data.archive_file.auto_teardown_zip.output_path
  source_code_hash = data.archive_file.auto_teardown_zip.output_base64sha256

  environment {
    variables = {
      MAX_UPTIME_HOURS = "4"
    }
  }

  # layers = [aws_lambda_layer_version.terraform_binary.arn]

  tags = { Project = "eks-control" }
}

# ─── CloudWatch Log Groups ───────────────────────────────────────────────────

resource "aws_cloudwatch_log_group" "cluster_control" {
  name              = "/aws/lambda/${aws_lambda_function.cluster_control.function_name}"
  retention_in_days = 14
}

resource "aws_cloudwatch_log_group" "cluster_status" {
  name              = "/aws/lambda/${aws_lambda_function.cluster_status.function_name}"
  retention_in_days = 14
}

resource "aws_cloudwatch_log_group" "auto_teardown" {
  name              = "/aws/lambda/${aws_lambda_function.auto_teardown.function_name}"
  retention_in_days = 14
}

resource "aws_lambda_function" "orchestrator" {
  function_name    = "accio-orchestrator"
  runtime          = "python3.12"
  handler          = "handler.lambda_handler"
  role             = aws_iam_role.accio_lambda.arn
  timeout          = 30

  filename         = "${path.module}/../lambdas/orchestrator/lambda.zip"
  source_code_hash = filebase64sha256("${path.module}/../lambdas/orchestrator/lambda.zip")

  environment {
    variables = {
      SFN_CREATE_ARN     = aws_sfn_state_machine.provision_cluster.arn
      CLUSTERS_TABLE     = aws_dynamodb_table.clusters.name
      CUSTOMER_ROLE_NAME = "accio-platform-provisioner"
      EXTERNAL_ID        = "accio-provisioner-v1"
    }
  }
}

resource "aws_lambda_permission" "api_gateway_orchestrator" {
  statement_id  = "AllowExecutionFromAPIGateway"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.orchestrator.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.accio.execution_arn}/*/*"
}

resource "aws_lambda_function" "start_codebuild" {
  function_name    = "accio-start-codebuild"
  runtime          = "python3.12"
  handler          = "handler.lambda_handler"
  role             = aws_iam_role.accio_lambda.arn
  timeout          = 30

  filename         = "${path.module}/../lambdas/start_codebuild/lambda.zip"
  source_code_hash = filebase64sha256("${path.module}/../lambdas/start_codebuild/lambda.zip")

  environment {
    variables = {
      CODEBUILD_PROJECT = aws_codebuild_project.provisioner.name
      TF_CODE_BUCKET    = data.aws_ssm_parameter.tf_code_bucket.value
      TF_STATE_BUCKET   = data.aws_ssm_parameter.tf_state_bucket.value
      OUTPUTS_BUCKET    = data.aws_ssm_parameter.outputs_bucket.value
    }
  }
}

# ---
# Data sources for SSM params (so we don't hardcode them or cause cycles)
# ---

data "aws_ssm_parameter" "tf_code_bucket" {
  name = "/accio/s3/tf-code-bucket"
  depends_on = [aws_ssm_parameter.tf_code_bucket]
}

data "aws_ssm_parameter" "tf_state_bucket" {
  name = "/accio/s3/tf-state-bucket"
  depends_on = [aws_ssm_parameter.tf_state_bucket]
}

data "aws_ssm_parameter" "outputs_bucket" {
  name = "/accio/s3/outputs-bucket"
  depends_on = [aws_ssm_parameter.outputs_bucket]
}

# ─── REST API Gateway ────────────────────────────────────────────────────────

resource "aws_api_gateway_rest_api" "eks_control" {
  name        = "eks-control-api"
  description = "EKS cluster control API for Flutter app"

  tags = { Project = "eks-control" }
}

# ─── /cluster resource ───────────────────────────────────────────────────────

resource "aws_api_gateway_resource" "cluster" {
  rest_api_id = aws_api_gateway_rest_api.eks_control.id
  parent_id   = aws_api_gateway_rest_api.eks_control.root_resource_id
  path_part   = "cluster"
}

# ─── /cluster/status ─────────────────────────────────────────────────────────

resource "aws_api_gateway_resource" "status" {
  rest_api_id = aws_api_gateway_rest_api.eks_control.id
  parent_id   = aws_api_gateway_resource.cluster.id
  path_part   = "status"
}

resource "aws_api_gateway_method" "get_status" {
  rest_api_id      = aws_api_gateway_rest_api.eks_control.id
  resource_id      = aws_api_gateway_resource.status.id
  http_method      = "GET"
  authorization    = "NONE"
  api_key_required = true
}

resource "aws_api_gateway_integration" "get_status" {
  rest_api_id             = aws_api_gateway_rest_api.eks_control.id
  resource_id             = aws_api_gateway_resource.status.id
  http_method             = aws_api_gateway_method.get_status.http_method
  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = aws_lambda_function.cluster_status.invoke_arn
}

resource "aws_lambda_permission" "apigw_status" {
  statement_id  = "AllowAPIGatewayStatus"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.cluster_status.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_api_gateway_rest_api.eks_control.execution_arn}/*/*"
}

# ─── /cluster/start ──────────────────────────────────────────────────────────

resource "aws_api_gateway_resource" "start" {
  rest_api_id = aws_api_gateway_rest_api.eks_control.id
  parent_id   = aws_api_gateway_resource.cluster.id
  path_part   = "start"
}

resource "aws_api_gateway_method" "post_start" {
  rest_api_id      = aws_api_gateway_rest_api.eks_control.id
  resource_id      = aws_api_gateway_resource.start.id
  http_method      = "POST"
  authorization    = "NONE"
  api_key_required = true
}

resource "aws_api_gateway_integration" "post_start" {
  rest_api_id             = aws_api_gateway_rest_api.eks_control.id
  resource_id             = aws_api_gateway_resource.start.id
  http_method             = aws_api_gateway_method.post_start.http_method
  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = aws_lambda_function.cluster_control.invoke_arn
}

resource "aws_lambda_permission" "apigw_start" {
  statement_id  = "AllowAPIGatewayStart"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.cluster_control.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_api_gateway_rest_api.eks_control.execution_arn}/*/*"
}

# ─── /cluster/stop ───────────────────────────────────────────────────────────

resource "aws_api_gateway_resource" "stop" {
  rest_api_id = aws_api_gateway_rest_api.eks_control.id
  parent_id   = aws_api_gateway_resource.cluster.id
  path_part   = "stop"
}

resource "aws_api_gateway_method" "post_stop" {
  rest_api_id      = aws_api_gateway_rest_api.eks_control.id
  resource_id      = aws_api_gateway_resource.stop.id
  http_method      = "POST"
  authorization    = "NONE"
  api_key_required = true
}

resource "aws_api_gateway_integration" "post_stop" {
  rest_api_id             = aws_api_gateway_rest_api.eks_control.id
  resource_id             = aws_api_gateway_resource.stop.id
  http_method             = aws_api_gateway_method.post_stop.http_method
  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = aws_lambda_function.cluster_control.invoke_arn
}

resource "aws_lambda_permission" "apigw_stop" {
  statement_id  = "AllowAPIGatewayStop"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.cluster_control.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_api_gateway_rest_api.eks_control.execution_arn}/*/*"
}

# ─── Deployment & Stage ──────────────────────────────────────────────────────

resource "aws_api_gateway_deployment" "prod" {
  rest_api_id = aws_api_gateway_rest_api.eks_control.id

  triggers = {
    redeployment = sha1(jsonencode([
      aws_api_gateway_resource.status.id,
      aws_api_gateway_resource.start.id,
      aws_api_gateway_resource.stop.id,
      aws_api_gateway_method.get_status.id,
      aws_api_gateway_method.post_start.id,
      aws_api_gateway_method.post_stop.id,
    ]))
  }

  lifecycle {
    create_before_destroy = true
  }

  depends_on = [
    aws_api_gateway_integration.get_status,
    aws_api_gateway_integration.post_start,
    aws_api_gateway_integration.post_stop,
  ]
}

resource "aws_api_gateway_stage" "prod" {
  deployment_id = aws_api_gateway_deployment.prod.id
  rest_api_id   = aws_api_gateway_rest_api.eks_control.id
  stage_name    = "prod"
}

# ─── API Key & Usage Plan (throttle: 10 req/min) ─────────────────────────────

resource "aws_api_gateway_api_key" "flutter_app" {
  name    = "eks-control-flutter-key"
  enabled = true
}

resource "aws_api_gateway_usage_plan" "flutter_app" {
  name = "eks-control-usage-plan"

  api_stages {
    api_id = aws_api_gateway_rest_api.eks_control.id
    stage  = aws_api_gateway_stage.prod.stage_name
  }

  throttle_settings {
    rate_limit  = 10   # requests per second
    burst_limit = 20
  }
}

resource "aws_api_gateway_usage_plan_key" "flutter_app" {
  key_id        = aws_api_gateway_api_key.flutter_app.id
  key_type      = "API_KEY"
  usage_plan_id = aws_api_gateway_usage_plan.flutter_app.id
}

# ─── Outputs ─────────────────────────────────────────────────────────────────

output "api_base_url" {
  description = "Base URL — paste this into api_service.dart"
  value       = "https://${aws_api_gateway_rest_api.eks_control.id}.execute-api.${var.region}.amazonaws.com/${aws_api_gateway_stage.prod.stage_name}"
}

output "api_key_id" {
  description = "API key ID — retrieve the value with: aws apigateway get-api-key --api-key <ID> --include-value"
  value       = aws_api_gateway_api_key.flutter_app.id
}

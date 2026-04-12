resource "aws_cloudwatch_event_rule" "auto_teardown" {
  name                = "eks-auto-teardown"
  description         = "Check cluster uptime every 15 min and destroy if > 4 hours"
  schedule_expression = "rate(15 minutes)"
}

resource "aws_cloudwatch_event_target" "teardown_lambda" {
  rule      = aws_cloudwatch_event_rule.auto_teardown.name
  target_id = "AutoTeardownLambda"
  arn       = aws_lambda_function.auto_teardown.arn
}

resource "aws_lambda_permission" "allow_eventbridge" {
  statement_id  = "AllowEventBridge"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.auto_teardown.function_name
  principal     = "events.amazonaws.com"
  source_arn    = aws_cloudwatch_event_rule.auto_teardown.arn
}

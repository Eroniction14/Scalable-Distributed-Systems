################################
# Lambda Function (uses LabRole from main.tf)
################################
resource "aws_lambda_function" "order_processor" {
  function_name = "hw7-order-processor"
  role          = var.lab_role_arn
  handler       = "bootstrap"
  runtime       = "provided.al2"
  memory_size   = 512
  timeout       = 30

  filename         = "lambda/lambda.zip"
  source_code_hash = filebase64sha256("lambda/lambda.zip")
}

################################
# SNS → Lambda subscription
################################
resource "aws_sns_topic_subscription" "lambda_sub" {
  topic_arn = aws_sns_topic.orders.arn
  protocol  = "lambda"
  endpoint  = aws_lambda_function.order_processor.arn
}

resource "aws_lambda_permission" "sns_invoke" {
  statement_id  = "AllowSNSInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.order_processor.function_name
  principal     = "sns.amazonaws.com"
  source_arn    = aws_sns_topic.orders.arn
}
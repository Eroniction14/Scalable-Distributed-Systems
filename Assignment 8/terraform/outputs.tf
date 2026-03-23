output "ecr_repository_url" {
  value = aws_ecr_repository.app.repository_url
}

output "rds_endpoint" {
  value = aws_db_instance.mysql.endpoint
}

output "dynamodb_table_name" {
  value = aws_dynamodb_table.shopping_carts.name
}

output "ecs_cluster_name" {
  value = aws_ecs_cluster.main.name
}

output "active_db" {
  value = var.active_db
}
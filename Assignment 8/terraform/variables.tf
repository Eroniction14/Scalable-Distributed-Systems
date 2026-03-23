variable "aws_region" {
  default = "us-east-1"
}

variable "db_username" {
  description = "RDS MySQL master username"
  default     = "admin"
}

variable "db_password" {
  description = "RDS MySQL master password"
  sensitive   = true
  default     = "ChangeMe123!" # Change this or pass via -var
}

variable "active_db" {
  description = "Which DB backend is active: 'mysql' or 'dynamodb'"
  default     = "mysql"
}
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.aws_region
}

# ============================================================================
# DATA SOURCES
# ============================================================================

data "aws_vpc" "default" {
  default = true
}

data "aws_subnets" "default" {
  filter {
    name   = "vpc-id"
    values = [data.aws_vpc.default.id]
  }
}

data "aws_iam_role" "lab_role" {
  name = "LabRole"
}

# ============================================================================
# SECURITY GROUPS
# ============================================================================

resource "aws_security_group" "ecs_sg" {
  name        = "hw8-ecs-sg"
  description = "ECS tasks - allow 8080 inbound, all outbound"
  vpc_id      = data.aws_vpc.default.id

  ingress {
    from_port   = 8080
    to_port     = 8080
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_security_group" "rds_sg" {
  name        = "hw8-rds-sg"
  description = "RDS MySQL - allow 3306 from ECS tasks only"
  vpc_id      = data.aws_vpc.default.id

  ingress {
    from_port       = 3306
    to_port         = 3306
    protocol        = "tcp"
    security_groups = [aws_security_group.ecs_sg.id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

# ============================================================================
# RDS MySQL (Step I)
# ============================================================================

resource "aws_db_subnet_group" "main" {
  name       = "hw8-db-subnet-group"
  subnet_ids = data.aws_subnets.default.ids

  tags = {
    Name = "hw8-db-subnet-group"
  }
}

resource "aws_db_instance" "mysql" {
  identifier             = "hw8-mysql"
  engine                 = "mysql"
  engine_version         = "8.0"
  instance_class         = "db.t3.micro"
  allocated_storage      = 20
  db_name                = "shopping"
  username               = var.db_username
  password               = var.db_password
  db_subnet_group_name   = aws_db_subnet_group.main.name
  vpc_security_group_ids = [aws_security_group.rds_sg.id]
  publicly_accessible    = false
  skip_final_snapshot    = true
  deletion_protection    = false

  tags = {
    Name = "hw8-mysql"
  }
}

# ============================================================================
# DynamoDB (Step II)
# ============================================================================

resource "aws_dynamodb_table" "shopping_carts" {
  name         = "shopping-carts"
  billing_mode = "PAY_PER_REQUEST" # On-demand = no capacity planning needed
  hash_key     = "id"

  attribute {
    name = "id"
    type = "S"
  }

  attribute {
    name = "customer_id"
    type = "S"
  }

  # GSI for "get all carts by customer" access pattern
  global_secondary_index {
    name            = "customer-index"
    hash_key        = "customer_id"
    projection_type = "ALL"
  }

  tags = {
    Name = "hw8-shopping-carts"
  }
}

# ============================================================================
# ECR
# ============================================================================

resource "aws_ecr_repository" "app" {
  name                 = "hw8-shopping-cart"
  image_tag_mutability = "MUTABLE"
  force_delete         = true
}

# ============================================================================
# CLOUDWATCH LOGS
# ============================================================================

resource "aws_cloudwatch_log_group" "app" {
  name              = "/ecs/hw8-shopping-cart"
  retention_in_days = 7
}

# ============================================================================
# ECS CLUSTER
# ============================================================================

resource "aws_ecs_cluster" "main" {
  name = "hw8-cluster"
}

# ============================================================================
# ECS TASK DEFINITION - MySQL variant
# ============================================================================

resource "aws_ecs_task_definition" "mysql" {
  family                   = "hw8-mysql"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = "256"
  memory                   = "512"
  execution_role_arn       = data.aws_iam_role.lab_role.arn
  task_role_arn            = data.aws_iam_role.lab_role.arn

  container_definitions = jsonencode([{
    name      = "app"
    image     = "${aws_ecr_repository.app.repository_url}:latest"
    essential = true
    portMappings = [{
      containerPort = 8080
      protocol      = "tcp"
    }]
    environment = [
      { name = "DB_TYPE", value = "mysql" },
      { name = "MYSQL_DSN", value = "${var.db_username}:${var.db_password}@tcp(${aws_db_instance.mysql.endpoint})/${aws_db_instance.mysql.db_name}" },
      { name = "PORT", value = "8080" },
    ]
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        "awslogs-group"         = aws_cloudwatch_log_group.app.name
        "awslogs-region"        = var.aws_region
        "awslogs-stream-prefix" = "mysql"
      }
    }
  }])
}

# ============================================================================
# ECS TASK DEFINITION - DynamoDB variant
# ============================================================================

resource "aws_ecs_task_definition" "dynamodb" {
  family                   = "hw8-dynamodb"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = "256"
  memory                   = "512"
  execution_role_arn       = data.aws_iam_role.lab_role.arn
  task_role_arn            = data.aws_iam_role.lab_role.arn

  container_definitions = jsonencode([{
    name      = "app"
    image     = "${aws_ecr_repository.app.repository_url}:latest"
    essential = true
    portMappings = [{
      containerPort = 8080
      protocol      = "tcp"
    }]
    environment = [
      { name = "DB_TYPE", value = "dynamodb" },
      { name = "DYNAMO_TABLE", value = aws_dynamodb_table.shopping_carts.name },
      { name = "AWS_REGION", value = var.aws_region },
      { name = "PORT", value = "8080" },
    ]
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        "awslogs-group"         = aws_cloudwatch_log_group.app.name
        "awslogs-region"        = var.aws_region
        "awslogs-stream-prefix" = "dynamodb"
      }
    }
  }])
}

# ============================================================================
# ECS SERVICE — deploy ONE at a time (toggle via variable)
# ============================================================================

resource "aws_ecs_service" "app" {
  name            = "hw8-service"
  cluster         = aws_ecs_cluster.main.id
  task_definition = var.active_db == "mysql" ? aws_ecs_task_definition.mysql.arn : aws_ecs_task_definition.dynamodb.arn
  desired_count   = 1
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = data.aws_subnets.default.ids
    security_groups  = [aws_security_group.ecs_sg.id]
    assign_public_ip = true
  }
}
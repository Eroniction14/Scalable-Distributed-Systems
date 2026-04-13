terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.0"
    }
  }
}

provider "aws" {
  region = "us-west-2"
}

variable "app_name" {
  default = "album-store"
}

variable "container_port" {
  default = 8080
}

# ─── VPC ────────────────────────────────────────────────────────────────────

resource "aws_vpc" "main" {
  cidr_block           = "10.0.0.0/16"
  enable_dns_support   = true
  enable_dns_hostnames = true

  tags = { Name = "${var.app_name}-vpc" }
}

resource "aws_internet_gateway" "main" {
  vpc_id = aws_vpc.main.id
  tags   = { Name = "${var.app_name}-igw" }
}

resource "aws_subnet" "public_a" {
  vpc_id                  = aws_vpc.main.id
  cidr_block              = "10.0.1.0/24"
  availability_zone       = "us-west-2a"
  map_public_ip_on_launch = true
  tags                    = { Name = "${var.app_name}-public-a" }
}

resource "aws_subnet" "public_b" {
  vpc_id                  = aws_vpc.main.id
  cidr_block              = "10.0.2.0/24"
  availability_zone       = "us-west-2b"
  map_public_ip_on_launch = true
  tags                    = { Name = "${var.app_name}-public-b" }
}

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.main.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.main.id
  }

  tags = { Name = "${var.app_name}-public-rt" }
}

resource "aws_route_table_association" "public_a" {
  subnet_id      = aws_subnet.public_a.id
  route_table_id = aws_route_table.public.id
}

resource "aws_route_table_association" "public_b" {
  subnet_id      = aws_subnet.public_b.id
  route_table_id = aws_route_table.public.id
}

# ─── Security Groups ───────────────────────────────────────────────────────

resource "aws_security_group" "alb" {
  name   = "${var.app_name}-alb-sg"
  vpc_id = aws_vpc.main.id

  ingress {
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = { Name = "${var.app_name}-alb-sg" }
}

resource "aws_security_group" "ecs" {
  name   = "${var.app_name}-ecs-sg"
  vpc_id = aws_vpc.main.id

  ingress {
    from_port       = var.container_port
    to_port         = var.container_port
    protocol        = "tcp"
    security_groups = [aws_security_group.alb.id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = { Name = "${var.app_name}-ecs-sg" }
}

# ─── ALB ────────────────────────────────────────────────────────────────────

resource "aws_lb" "main" {
  name               = "${var.app_name}-alb"
  internal           = false
  load_balancer_type = "application"
  security_groups    = [aws_security_group.alb.id]
  subnets            = [aws_subnet.public_a.id, aws_subnet.public_b.id]

  tags = { Name = "${var.app_name}-alb" }
}

resource "aws_lb_target_group" "main" {
  name        = "${var.app_name}-tg"
  port        = var.container_port
  protocol    = "HTTP"
  vpc_id      = aws_vpc.main.id
  target_type = "ip"

  health_check {
    path                = "/health"
    interval            = 15
    timeout             = 5
    healthy_threshold   = 2
    unhealthy_threshold = 3
    matcher             = "200"
  }

  tags = { Name = "${var.app_name}-tg" }
}

resource "aws_lb_listener" "http" {
  load_balancer_arn = aws_lb.main.arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.main.arn
  }
}

# ─── DynamoDB ───────────────────────────────────────────────────────────────

resource "aws_dynamodb_table" "albums" {
  name         = "Albums"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "album_id"

  attribute {
    name = "album_id"
    type = "S"
  }

  tags = { Name = "${var.app_name}-albums" }
}

resource "aws_dynamodb_table" "photos" {
  name         = "Photos"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "photo_id"

  attribute {
    name = "photo_id"
    type = "S"
  }

  tags = { Name = "${var.app_name}-photos" }
}

# ─── S3 ─────────────────────────────────────────────────────────────────────

resource "random_id" "suffix" {
  byte_length = 4
}

resource "aws_s3_bucket" "photos" {
  bucket        = "${var.app_name}-photos-${random_id.suffix.hex}"
  force_destroy = true

  tags = { Name = "${var.app_name}-photos" }
}

resource "aws_s3_bucket_public_access_block" "photos" {
  bucket = aws_s3_bucket.photos.id

  block_public_acls       = false
  block_public_policy     = false
  ignore_public_acls      = false
  restrict_public_buckets = false
}

resource "aws_s3_bucket_policy" "photos_public_read" {
  bucket = aws_s3_bucket.photos.id

  depends_on = [aws_s3_bucket_public_access_block.photos]

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid       = "PublicReadPhotos"
        Effect    = "Allow"
        Principal = "*"
        Action    = "s3:GetObject"
        Resource  = "${aws_s3_bucket.photos.arn}/photos/*"
      }
    ]
  })
}

# ─── SQS ────────────────────────────────────────────────────────────────────

resource "aws_sqs_queue" "photo_processing" {
  name                       = "${var.app_name}-photo-processing"
  visibility_timeout_seconds = 60
  message_retention_seconds  = 86400
  receive_wait_time_seconds  = 20

  tags = { Name = "${var.app_name}-photo-processing" }
}

# ─── ECR ────────────────────────────────────────────────────────────────────

resource "aws_ecr_repository" "main" {
  name         = var.app_name
  force_delete = true

  tags = { Name = "${var.app_name}-ecr" }
}

# ─── IAM ────────────────────────────────────────────────────────────────────

data "aws_iam_role" "lab_role" {
  name = "LabRole"
}

# ─── ECS ────────────────────────────────────────────────────────────────────

resource "aws_ecs_cluster" "main" {
  name = "${var.app_name}-cluster"
  tags = { Name = "${var.app_name}-cluster" }
}

resource "aws_cloudwatch_log_group" "ecs" {
  name              = "/ecs/${var.app_name}"
  retention_in_days = 7
  tags              = { Name = "${var.app_name}-logs" }
}

resource "aws_ecs_task_definition" "main" {
  family                   = var.app_name
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = "2048"
  memory                   = "4096"
  execution_role_arn       = data.aws_iam_role.lab_role.arn
  task_role_arn            = data.aws_iam_role.lab_role.arn

  container_definitions = jsonencode([
    {
      name      = var.app_name
      image     = "${aws_ecr_repository.main.repository_url}:latest"
      essential = true

      portMappings = [
        {
          containerPort = var.container_port
          protocol      = "tcp"
        }
      ]

      environment = [
        { name = "PORT", value = tostring(var.container_port) },
        { name = "ALBUMS_TABLE", value = aws_dynamodb_table.albums.name },
        { name = "PHOTOS_TABLE", value = aws_dynamodb_table.photos.name },
        { name = "S3_BUCKET", value = aws_s3_bucket.photos.id },
        { name = "SQS_QUEUE_URL", value = aws_sqs_queue.photo_processing.url },
        { name = "AWS_REGION", value = "us-west-2" },
      ]

      logConfiguration = {
        logDriver = "awslogs"
        options = {
          "awslogs-group"         = "/ecs/${var.app_name}"
          "awslogs-region"        = "us-west-2"
          "awslogs-stream-prefix" = "ecs"
        }
      }
    }
  ])
}

resource "aws_ecs_service" "main" {
  name            = var.app_name
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.main.arn
  desired_count   = 2
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = [aws_subnet.public_a.id, aws_subnet.public_b.id]
    security_groups  = [aws_security_group.ecs.id]
    assign_public_ip = true
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.main.arn
    container_name   = var.app_name
    container_port   = var.container_port
  }

  depends_on = [aws_lb_listener.http]
}

# ─── Outputs ────────────────────────────────────────────────────────────────

output "alb_url" {
  value       = "http://${aws_lb.main.dns_name}"
  description = "The base URL to submit to ChaosArena"
}

output "ecr_repository_url" {
  value       = aws_ecr_repository.main.repository_url
  description = "Push your Docker image here"
}

output "s3_bucket" {
  value = aws_s3_bucket.photos.id
}
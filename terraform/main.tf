data "aws_vpc" "default" {
  default = true
}

data "aws_route53_zone" "selected" {
  name         = var.domain_name
  private_zone = false
}

## ACM CERT

resource "aws_acm_certificate" "cert" {
  provider          = aws.us_east_1
  domain_name       = var.domain_name
  validation_method = "DNS"

  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_route53_record" "cert_validation" {
  for_each = {
    for dvo in aws_acm_certificate.cert.domain_validation_options : dvo.domain_name => {
      name  = dvo.resource_record_name
      type  = dvo.resource_record_type
      value = dvo.resource_record_value
    }
  }
  zone_id = data.aws_route53_zone.selected.zone_id
  name    = each.value.name
  type    = each.value.type
  ttl     = 300
  records = [each.value.value]
}

resource "aws_acm_certificate_validation" "cert_validation" {
  provider                = aws.us_east_1
  certificate_arn         = aws_acm_certificate.cert.arn
  validation_record_fqdns = [for record in aws_route53_record.cert_validation : record.fqdn]
}

## ACM CERT

#
# resource "aws_route53_zone" "site" {
#   name = var.domain_name
# }
#
# resource "aws_route53_record" "site_alias" {
#   zone_id = aws_route53_zone.site.zone_id
#   name    = var.domain_name
#   type    = "A"
#
#   // TODO: add here ECS service of builder application
#   alias {
#     name                   = "s3-website-us-east-1.amazonaws.com"
#     zone_id                = "Z3AQBSTGFYJSTF" # S3 website endpoint hosted zone ID
#     evaluate_target_health = false
#   }
# }

resource "aws_subnet" "public_subnet" {
  vpc_id            = data.aws_vpc.default.id
  cidr_block        = data.aws_vpc.default.cidr_block
  map_public_ip_on_launch = true
}

resource "aws_security_group" "ecs_sg" {
  vpc_id = data.aws_vpc.default.id
  #
  #   ingress {
  #     from_port   = 80
  #     to_port     = 80
  #     protocol    = "tcp"
  #     cidr_blocks = ["0.0.0.0/0"]
  #   }
  #
  #   egress {
  #     from_port   = 0
  #     to_port     = 0
  #     protocol    = "-1"
  #     cidr_blocks = ["0.0.0.0/0"]
  #   }
}

resource "aws_ecs_cluster" "test_cluster" {
  name = "test_cluster"
}

resource "aws_ecs_task_definition" "keycloak" {
  family                   = "keycloak"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  #   execution_role_arn       = aws_iam_role.ecs_task_execution_role.arn
  #   task_role_arn            = aws_iam_role.ecs_container_role.arn
  memory                   = "512"
  cpu                      = "256"
  runtime_platform {
    operating_system_family = "LINUX"
    cpu_architecture        = "X86_64"
  }

  container_definitions = jsonencode([
    {
      name      = "keycloak"
      image     = "quay.io/keycloak/keycloak:26.2.0"
      command = ["start-dev", "--https-port", "7080", "-Djava.net.preferIPv4Stack=true"]
      essential = true
      #       logConfiguration = {
      #         logDriver = "awslogs"
      #         options = {
      #           awslogs-group         = aws_cloudwatch_log_group.payments-task-log-group.name
      #           awslogs-region        = "${var.cloud_region}"
      #           awslogs-stream-prefix = "ecs"
      #         }
      #       }
      portMappings = [
        {
          containerPort = 7080
          hostPort      = 7080
        }
      ]
      environment = [
        { name = "KC_DB",               value = "postgres" },
        { name = "KC_DB_URL",           value = "jdbc:postgresql://${aws_db_instance.postgres.address}:${aws_db_instance.postgres.port}/${aws_db_instance.postgres.db_name}" },
        { name = "KC_DB_USERNAME",      value = var.db_username },
        { name = "KC_DB_PASSWORD",      value = var.db_password },
        { name = "KC_DB_SCHEMA",        value = "keycloak" },
        { name = "KC_DB_POOL_MAX_SIZE", value = "5" }
      ]
    }
  ])
}

resource "aws_ecs_service" "keycloak-service" {
  name            = "keycloak-service"
  cluster         = aws_ecs_cluster.test_cluster.id
  task_definition = aws_ecs_task_definition.keycloak.arn
  desired_count   = 1
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = [aws_subnet.public_subnet.id]
    security_groups  = [aws_security_group.ecs_sg.id]
    assign_public_ip = true
  }

  depends_on = [
    aws_db_instance.postgres
  ]
}

resource "aws_s3_bucket" "sanity-web" {
  bucket = "sanity-web"
}

resource "aws_s3_bucket_public_access_block" "public_access" {
  bucket = aws_s3_bucket.sanity-web.id
  block_public_acls       = false
  block_public_policy     = false
  ignore_public_acls      = false
  restrict_public_buckets = false
}

resource "aws_s3_bucket_policy" "public_read" {
  depends_on = [aws_s3_bucket_public_access_block.public_access]
  bucket = aws_s3_bucket.sanity-web.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect    = "Allow"
        Principal = "*"
        Action    = ["s3:GetObject"]
        Resource  = "${aws_s3_bucket.sanity-web.arn}/*"
      }
    ]
  })
}
